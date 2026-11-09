package main

import (
	"SmartRun/cmd/server"
	db "SmartRun/internal/DB"
	"SmartRun/internal/adapter/importer/fit"
	"SmartRun/internal/adapter/llm"
	"SmartRun/internal/broker"
	"SmartRun/internal/broker/rabbitmq"
	"SmartRun/internal/cache"
	"SmartRun/internal/config"
	myhttp "SmartRun/internal/handler/http"
	"SmartRun/internal/logger"
	"SmartRun/internal/repository/postgres"
	workoutpostgres "SmartRun/internal/repository/postgres/workout"
	"SmartRun/internal/usecase/service"
	"SmartRun/internal/worker"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-playground/validator"
	"github.com/go-webauthn/webauthn/webauthn"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	log, logCloser := logger.NewLogger(cfg.Logger)
	defer func() {
		_ = logCloser()
	}()
	ctx := logger.WithContext(context.Background(), log) // создаём базовый контекст с логгером
	err = run(ctx, log, cfg)
	if err != nil {
		log.Error("failed to start app", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, cfg *config.Config) error {
	pool, err := db.NewPool(ctx, cfg.Postgres.ConnString())
	if err != nil {
		log.Error("failed to create database pool", "error", err,
			"stack", string(debug.Stack()))
		return err
	}
	defer pool.Close()
	validator := validator.New()
	cacheStore := cache.NewNoopCache()
	if cfg.Redis.URL != "" {
		if redisCache, cacheErr := cache.NewRedisCache(cfg.Redis.URL); cacheErr != nil {
			log.Warn("redis cache unavailable, fallback to noop cache", "error", cacheErr)
		} else {
			cacheStore = redisCache
			log.Info("redis cache initialized")
		}
	}
	defer func() {
		if err = cacheStore.Close(); err != nil {
			log.Warn("cache close error", "error", err)
		}
	}()

	userRepo := postgres.NewUserRepository(pool, cacheStore)
	sessionRepo := postgres.NewSessionRepository(pool)
	totpRepo := postgres.NewTOTPRepository(pool)
	authCfg := cfg.Auth
	txManager := postgres.NewTxManager(pool)
	passwResetRepo := postgres.NewPasswordResetRepository(pool)
	emailService, err := service.NewEmailService(authCfg.Email)
	if err != nil {
		log.Error("failed to create email service", "error", err,
			"stack", string(debug.Stack()))
		return err
	}

	rabbitURL := cfg.RabbitMQ.URL
	var publisher broker.Publisher = broker.NewNoopPublisher()
	if rabbitPublisher, rabbitErr := rabbitmq.NewPublisher(rabbitURL); rabbitErr != nil {
		log.Warn("rabbitmq publisher unavailable, fallback to noop publisher (async events disabled)", "error", rabbitErr)
	} else {
		publisher = rabbitPublisher
		defer rabbitPublisher.Close()
		log.Info("rabbitmq publisher initialized")
	}

	passkeyRepo := postgres.NewPasskeyRepository(pool)

	userService := service.NewUserService(userRepo, sessionRepo, totpRepo, passwResetRepo, emailService, authCfg, txManager, validator, publisher)
	userHandler := myhttp.NewUserHandler(userService)

	workoutRepo := workoutpostgres.NewWorkoutRepository(pool)
	dailyMetricsRepo := postgres.NewDailyMetricRepository(pool)
	parser := fit.NewMuktihariFitParser()
	workoutService := service.NewWorkoutService(workoutRepo, dailyMetricsRepo, userRepo, parser, validator, txManager, publisher)
	workoutHandler := myhttp.NewWorkoutHandler(workoutService)

	dailyMetricsService := service.NewDailyMetricService(dailyMetricsRepo, workoutRepo, validator, txManager, publisher)
	dailyMetricsHandler := myhttp.NewDailyMetricHandler(dailyMetricsService)

	// Конфигурация FIDO2 / WebAuthn (Passkeys)
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.WebAuthn.RPDisplayName,
		RPID:          cfg.WebAuthn.RPID,
		RPOrigins:     cfg.WebAuthn.RPOrigins,
	})
	if err != nil {
		log.Error("failed to initialize webauthn", "error", err, "stack", string(debug.Stack()))
		return err
	}

	passkeyService := service.NewPasskeyService(userRepo, passkeyRepo, sessionRepo, totpRepo, txManager, cacheStore, webAuthn, authCfg)
	passkeyHandler := myhttp.NewPasskeyHandler(passkeyService)

	if workerPool, poolErr := worker.NewWorkerPool(rabbitURL); poolErr != nil {
		log.Warn("rabbitmq worker pool unavailable, background workers disabled", "error", poolErr)
	} else {
		defer workerPool.Close()

		metricsWorker := worker.NewMetricsWorker(workoutService)
		if err = workerPool.StartConsumer(rabbitmq.MetricsQueue, metricsWorker); err != nil {
			log.Error("failed to start metrics worker", "error", err)
			return err
		}

		emailWorker := worker.NewEmailWorker(emailService)
		if err = workerPool.StartConsumer(rabbitmq.PasswordResetQueue, emailWorker); err != nil {
			log.Error("failed to start email worker", "error", err)
			return err
		}

		aiCoach, err := llm.NewGeminiCoach(context.Background())
		if err != nil {
			log.Warn("failed to initialize AI coach, advice generation will be disabled", "error", err)
		} else {
			aiCoachWorker := worker.NewAICoachWorker(aiCoach, dailyMetricsService, workoutService)
			if err = workerPool.StartConsumer(rabbitmq.AICoachQueue, aiCoachWorker); err != nil {
				log.Error("failed to start ai coach worker", "error", err)
			}
		}
	}

	metricsRepo := postgres.NewMetricsRepository(pool, cacheStore)
	metricsService := service.NewMetricsService(metricsRepo, validator)
	metricsHandler := myhttp.NewMetricHandler(metricsService)

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Error("cannot create scheduler: %v", "error", err,
			"stack", string(debug.Stack()))
		return err
	}
	job, err := s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 0, 0))), // Каждый день в 3:00
		gocron.NewTask(func() {
			err = sessionRepo.CleanupExpiredSessions(context.Background())
			if err != nil {
				log.Error("cleanup error:", "error", err,
					"stack", string(debug.Stack()))
			}
		}),
	)
	if err != nil {
		log.Error("error created tasks: %v", "error", err)
		return err
	}
	log.Info("tasks succefully created with ID: %s", "signal", job.ID().String())
	s.Start()

	defer func() {
		if err := s.Shutdown(); err != nil {
			log.Error("scheduler shutdown error", "error", err,
				"stack", string(debug.Stack()))
		} else {
			log.Info("scheduler shut down successfully")
		}
	}()

	// Создаём HTTP сервер
	handler := server.NewServer(userHandler,
		workoutHandler,
		metricsHandler,
		dailyMetricsHandler,
		passkeyHandler,
		cfg.Auth.JWTSecret,
		ctx)
	httpServer := &http.Server{
		Addr:              cfg.HTTP.Address(),
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ReadHeaderTimeout: 3 * time.Second,
	}

	serverErrors := make(chan error, 1)
	// Запускаем сервер в горутине
	go func() {
		msg := fmt.Sprintf("starting server on %s", httpServer.Addr)
		log.Info(msg)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server listen error: %v", "error", err,
				"stack", string(debug.Stack()))
			serverErrors <- err
		}
	}()

	// Ждём сигнала завершения (Ctrl+C или SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		log.Error("fail to start server: %v", "error", err)
		return err
	case sig := <-quit:
		log.Info("received shutdown signal", "signal", sig)
	}

	// Graceful shutdown с таймаутом 30 сек
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", "error", err,
			"stack", string(debug.Stack()))
		return err
	}
	log.Info("HTTP server shutdown successfully")

	log.Info("application stopped")
	return nil
}
