package server

import (
	myhttp "SmartRun/internal/handler/http"
	my_middleware "SmartRun/internal/middleware/auth"
	ratelimit "SmartRun/internal/middleware/ratelimit"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

/*func RateLimit(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Every(time.Minute), 5) // 5 запросов в минуту
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}*/

var (
	authLimiter = ratelimit.NewRateLimitMiddleware(ratelimit.RateLimitConfig{
		Requests: 5,
		Window:   time.Minute,
		KeyBy:    []string{"real_ip"}, // только по реальному IP
	})

	loginLimiter = ratelimit.NewRateLimitMiddleware(ratelimit.RateLimitConfig{
		Requests: 10,
		Window:   time.Minute,
		KeyBy:    []string{"real_ip"},
	})

	userLimiter = ratelimit.NewRateLimitMiddleware(ratelimit.RateLimitConfig{
		Requests: 300,
		Window:   time.Minute,
		KeyBy:    []string{"user_id", "endpoint"}, // по пользователю + путь
	})
)

func NewServer(userHandler *myhttp.UserHandler,
	workoutHandler *myhttp.WorkoutHandler,
	metricsHandler *myhttp.MetricHandler,
	dailyMetricsHandler *myhttp.DailyMetricHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	secret := os.Getenv("JWT_SECRET")
	auth := my_middleware.NewAuthMiddleware(secret)

	r.Route("/api", func(r chi.Router) {

		r.With(authLimiter).Post("/register", userHandler.Register)
		r.With(loginLimiter).Post("/login", http.HandlerFunc(userHandler.Login).ServeHTTP)
		r.With(authLimiter).Post("/refresh", http.HandlerFunc(userHandler.Refresh).ServeHTTP)
		r.With(authLimiter).Post("/password/reset/request", userHandler.RequestPasswordReset)
		r.With(authLimiter).Post("/password/reset/confirm", userHandler.ResetPassword)

		// Verify2FA НЕ требует JWT - она используется в ПРОЦЕССЕ аутентификации
		r.Post("/verify-2fa", userHandler.Verify2FA)

		r.Group(func(r chi.Router) {
			r.Use(auth.JWT)
			r.With(userLimiter).Get("/me", userHandler.Me)
			r.Post("/enable-2fa", userHandler.Enable2FA)

			r.With(userLimiter).Post("/workouts", workoutHandler.Create)
			r.With(userLimiter).Get("/workouts", workoutHandler.GetAll)
			r.With(userLimiter).Get("/workouts/history", workoutHandler.GetHistoryByMonth)
			r.With(userLimiter).Get("/workouts/{id}", workoutHandler.GetByID)
			r.With(userLimiter).Patch("/workouts/{id}", workoutHandler.Update)
			r.With(userLimiter).Put("/workouts/{id}", workoutHandler.Update)
			r.With(userLimiter).Delete("/workouts/{id}", workoutHandler.Delete)

			r.With(userLimiter).Get("/metrics", metricsHandler.GetMetrics)
			r.With(userLimiter).Post("/metrics", metricsHandler.CreateMetrics)
			r.With(userLimiter).Delete("/metrics", metricsHandler.DeleteMetrics)
			r.With(userLimiter).Get("/metrics", metricsHandler.GetStoredMetrics)
			r.With(userLimiter).Put("/metrics", metricsHandler.UpdateMetrics)

			r.With(userLimiter).Get("/daily-metrics", dailyMetricsHandler.GetDailyMetrics)
			r.With(userLimiter).Post("/daily-metrics", dailyMetricsHandler.CreateDailyMetric)
			r.With(userLimiter).Put("/daily-metrics/{id}", dailyMetricsHandler.UpdateDailyMetric)
			r.With(userLimiter).Get("/daily-metrics/{id}", dailyMetricsHandler.GetDailyMetricByID)
			r.With(userLimiter).Delete("/daily-metrics/{id}", dailyMetricsHandler.DeleteDailyMetric)
		})
	})

	return r
}
