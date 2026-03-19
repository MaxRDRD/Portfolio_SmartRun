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
	userService := service.NewUserService(userRepo, sessionRepo, totpRepo, cfg, txManager)
	userHandler := myhttp.NewUserHandler(userService)

	server := server.NewServer(userHandler)

	log.Fatal(http.ListenAndServe(":8080", server))
}
