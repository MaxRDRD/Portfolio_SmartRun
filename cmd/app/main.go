package main

import (
	"SmartRun/cmd/server"
	db "SmartRun/internal/DB"
	"SmartRun/internal/config"
	"SmartRun/internal/usecase/service"
	"time"

	myhttp "SmartRun/internal/handler/http"
	repopostgres "SmartRun/internal/repository_impl/postgres"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-playground/validator"
)

func main() {
	ctx := context.Background()
	conn_string := os.Getenv("CONN_STRING")
	pool, err := db.NewPool(ctx, conn_string)
	if err != nil {
		panic(err)
	}

	userRepo := repopostgres.NewUserRepository(pool)
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
		log.Fatal(err)
	}
	userService := service.NewUserService(userRepo, sessionRepo, totpRepo, passwResetRepo, emailService, cfg, txManager)
	userHandler := myhttp.NewUserHandler(userService)

	workoutRepo := repopostgres.NewWorkoutRepository(pool)
	hrZonesRepo := repopostgres.NewHRZonesRepository(pool)
	validate := validator.New()
	workoutService := service.NewWorkoutService(workoutRepo, userRepo, hrZonesRepo, validate)
	workoutHandler := myhttp.NewWorkoutHandler(workoutService)

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatalf("cannot create scheduler: %v", err)
		// или: return err (если main возвращает error в некоторых шаблонах)
	}
	s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 0, 0))), // Каждый день в 3:00
		gocron.NewTask(func() {
			err := sessionRepo.CleanupExpiredSessions(context.Background())
			if err != nil {
				log.Println("cleanup error:", err)
			}
		}),
	)
	s.Start()

	server := server.NewServer(userHandler, workoutHandler)

	log.Fatal(http.ListenAndServe(":8080", server))

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := s.Shutdown(); err != nil {
		log.Printf("scheduler shutdown error: %v", err)
	}
}
