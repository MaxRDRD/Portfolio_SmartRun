package server

import (
	"SmartRun/internal/user"
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const userIDKey contextKey = "userID"

func GetUserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

func NewServer(userHandler *user.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	secret := os.Getenv("SECRET_KEY")
	auth := NewMiddleware(secret)

	r.Route("/api", func(r chi.Router) {

		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(auth.JWT)
			r.Get("/me", userHandler.Me)
		})

		r.Post("/workouts", nil)
		r.Get("/workouts", nil)
		r.Get("/workouts/{id}", nil)
		r.Delete("/workouts/{id}", nil)
	})
	return r
}
