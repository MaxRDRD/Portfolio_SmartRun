package server

import (
	myhttp "SmartRun/internal/handler/http"
	"SmartRun/pkg/my_middleware"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

func RateLimit(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Every(time.Minute), 5) // 5 запросов в минуту
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NewServer(userHandler *myhttp.UserHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	secret := os.Getenv("SECRET_KEY")
	auth := my_middleware.NewMiddleware(secret)

	r.Route("/api", func(r chi.Router) {

		r.Post("/register", userHandler.Register)
		r.Post("/login", RateLimit(http.HandlerFunc(userHandler.Login)).ServeHTTP)
		r.Post("/refresh", RateLimit(http.HandlerFunc(userHandler.Refresh)).ServeHTTP)

		r.Group(func(r chi.Router) {
			r.Use(auth.JWT)
			r.Get("/me", userHandler.Me)
			r.Post("/enable-2fa", userHandler.Enable2FA)
			r.Post("/verify-2fa", userHandler.Verify2FA)
		})

		r.Post("/workouts", nil)
		r.Get("/workouts", nil)
		r.Get("/workouts/{id}", nil)
		r.Delete("/workouts/{id}", nil)
	})
	return r
}
