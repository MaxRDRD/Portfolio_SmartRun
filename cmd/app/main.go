package main

import (
	"SmartRun/cmd/server"
	db "SmartRun/internal/DB"
	"SmartRun/internal/adapter/importer/fit"
	"SmartRun/internal/adapter/llm"
	"SmartRun/internal/broker/rabbitmq"
	"SmartRun/internal/cache"
	"SmartRun/internal/config"
	myhttp "SmartRun/internal/handler/http"
	"SmartRun/internal/logger"
	"SmartRun/internal/middleware"
	repopostgres "SmartRun/internal/repository_impl/postgres"
	workoutpostgres "SmartRun/internal/repository_impl/postgres/workout"
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
)

func main() {
	log := logger.NewLogger()
	ctx := logger.WithContext(context.Background(), log) // создаём базовый контекст с логгером
	err := run(ctx, log)
	if err != nil {
		log.Error("failed to start app", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	conn_string := os.Getenv("CONN_STRING")
	pool, err := db.NewPool(ctx, conn_string)
	if err != nil {
		log.Error("failed to create database pool", "error", err,
			"stack", string(debug.Stack()))
		return err
	}
	defer pool.Close()
	validator := validator.New()
	cacheStore := cache.NewNoopCache()
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		if redisCache, cacheErr := cache.NewRedisCache(redisURL); cacheErr != nil {
			log.Warn("redis cache unavailable, fallback to noop cache", "error", cacheErr)
		} else {
			cacheStore = redisCache
			log.Info("redis cache initialized")
		}
	}

	userRepo := repopostgres.NewUserRepository(pool, cacheStore)
	sessionRepo := repopostgres.NewSessionRepository(pool)
	totpRepo := repopostgres.NewTOTPRepository(pool)
	cfg := config.AuthConfig{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		PublicURL:       os.Getenv("APP_PUBLIC_URL"),
	}
	if cfg.PublicURL == "" {
		cfg.PublicURL = "http://localhost:3000"
	}
	txManager := repopostgres.NewTxManager(pool)
	passwResetRepo := repopostgres.NewPasswordResetRepository(pool)
	emailService, err := service.NewEmailService(cfg.Email)
	if err != nil {
		log.Error("failed to create email service", "error", err,
			"stack", string(debug.Stack()))
		return err
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://smartrun:smartrun@localhost:5672/"
	}
	publisher, err := rabbitmq.NewRabbitMQPublisher(rabbitURL)
	if err != nil {
		log.Error("failed to initialize rabbitmq publisher", "error", err, "stack", string(debug.Stack()))
		return err
	}
	defer publisher.Close()

	userService := service.NewUserService(userRepo, sessionRepo, totpRepo, passwResetRepo, emailService, cfg, txManager, validator, publisher)
	userHandler := myhttp.NewUserHandler(userService)

	workoutRepo := workoutpostgres.NewWorkoutRepository(pool)
	dailyMetricsRepo := repopostgres.NewDailyMetricRepository(pool)
	parser := fit.NewMuktihariFitParser()
	workoutService := service.NewWorkoutService(workoutRepo, dailyMetricsRepo, userRepo, parser, validator, txManager, publisher)
	workoutHandler := myhttp.NewWorkoutHandler(workoutService)

	dailyMetricsService := service.NewDailyMetricService(dailyMetricsRepo, workoutRepo, validator, txManager, publisher)
	dailyMetricsHandler := myhttp.NewDailyMetricHandler(dailyMetricsService)

	workerPool, err := worker.NewWorkerPool(rabbitURL)
	if err != nil {
		log.Error("failed to initialize worker pool", "error", err, "stack", string(debug.Stack()))
		return err
	}
	defer workerPool.Close()

	metricsWorker := worker.NewMetricsWorker(workoutService)
	if err := workerPool.StartConsumer(rabbitmq.MetricsQueue, metricsWorker); err != nil {
		log.Error("failed to start metrics worker", "error", err)
		return err
	}

	emailWorker := worker.NewEmailWorker(emailService)
	if err := workerPool.StartConsumer(rabbitmq.PasswordResetQueue, emailWorker); err != nil {
		log.Error("failed to start email worker", "error", err)
		return err
	}

	aiCoach, err := llm.NewGeminiCoach(context.Background())
	if err != nil {
		log.Warn("failed to initialize AI coach, advice generation will be disabled", "error", err)
	} else {
		aiCoachWorker := worker.NewAICoachWorker(aiCoach, dailyMetricsService, workoutService)
		if err := workerPool.StartConsumer(rabbitmq.AICoachQueue, aiCoachWorker); err != nil {
			log.Error("failed to start ai coach worker", "error", err)
		}
	}

	metricsRepo := repopostgres.NewMetricsRepository(pool, cacheStore)
	metricsService := service.NewMetricsService(metricsRepo, validator)
	metricsHandler := myhttp.NewMetricHandler(metricsService)

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Error("cannot create scheduler: %v", "error", err,
			"stack", string(debug.Stack()))
		return err
	}
	s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 0, 0))), // Каждый день в 3:00
		gocron.NewTask(func() {
			err := sessionRepo.CleanupExpiredSessions(context.Background())
			if err != nil {
				log.Error("cleanup error:", "error", err,
					"stack", string(debug.Stack()))
			}
		}),
	)
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
		ctx)
	// Оборачиваем в middleware, чтобы в каждом запросе был логгер с request_id и т.д.
	loggedHandler := middleware.LogMiddleware(log)(handler)
	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      loggedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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
	} else {
		log.Info("HTTP server shutdown successfully")
	}

	log.Info("application stopped")
	return nil
}
