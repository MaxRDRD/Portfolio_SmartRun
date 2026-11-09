package server

import (
	myhttp "SmartRun/internal/handler/http"
	"SmartRun/internal/logger"
	appmiddleware "SmartRun/internal/middleware"
	authmiddleware "SmartRun/internal/middleware/auth"
	ratelimit "SmartRun/internal/middleware/ratelimit"
	"context"
	"net/http"
	"os"
	"strings"
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
	dailyMetricsHandler *myhttp.DailyMetricHandler,
	passkeyHandler *myhttp.PasskeyHandler,
	jwtSecret string,
	ctx context.Context) http.Handler {
	log := logger.FromContext(ctx)
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(appmiddleware.LogMiddleware(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(appmiddleware.BodyLimit(1 << 20))

	auth := authmiddleware.NewAuthMiddleware(jwtSecret)

	// Serve existing frontend static files from ui/static.
	staticFS := http.FileServer(http.Dir("ui/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", staticFS))

	// Serve modern React frontend build if present under /app/
	if _, err := os.Stat("frontend/dist"); err == nil {
		appFS := http.FileServer(http.Dir("frontend/dist"))
		r.Handle("/app/*", http.StripPrefix("/app/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cleanPath := strings.TrimPrefix(r.URL.Path, "/")
			if cleanPath != "" {
				if _, err := os.Stat("frontend/dist/" + cleanPath); err == nil {
					appFS.ServeHTTP(w, r)
					return
				}
			}
			http.ServeFile(w, r, "frontend/dist/index.html")
		})))
		r.Get("/app", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/app/", http.StatusFound)
		})
	}

	// Basic frontend routes.
	r.Get("/", serveFrontendIndex)
	r.Get("/dashboard", serveFrontendDashboard)
	r.Get("/workouts", serveFrontendWorkouts)
	r.Get("/workouts/{id}", serveFrontendWorkoutDetails)
	r.Get("/metrics/analytics", serveFrontendMetricsAnalytics)
	r.Get("/daily-metrics", serveFrontendDailyMetrics)
	r.Get("/metrics/stored", serveFrontendStoredMetrics)
	r.Get("/login", serveFrontendLogin)
	r.Get("/signup", serveFrontendSignup)
	r.Get("/profile", serveFrontendProfile)
	r.Get("/about", serveFrontendIndex)

	r.Route("/api", func(r chi.Router) {

		r.With(authLimiter).Post("/register", userHandler.Register)
		r.With(loginLimiter).Post("/login", http.HandlerFunc(userHandler.Login).ServeHTTP)
		r.With(authLimiter).Post("/refresh", http.HandlerFunc(userHandler.Refresh).ServeHTTP)
		r.With(authLimiter).Post("/password/reset/request", userHandler.RequestPasswordReset)
		r.With(authLimiter).Post("/password/reset/confirm", userHandler.ResetPassword)
		r.With(authLimiter).Post("/auth/passkeys/login/begin", passkeyHandler.BeginLogin)
		r.With(authLimiter).Post("/auth/passkeys/login/finish", passkeyHandler.FinishLogin)
		// Verify2FA НЕ требует JWT - она используется в ПРОЦЕССЕ аутентификации
		r.Post("/verify-2fa", userHandler.Verify2FA)

		r.Group(func(r chi.Router) {
			r.Use(auth.JWT)
			r.With(userLimiter).Get("/me", userHandler.Me)
			r.With(userLimiter).Patch("/me", userHandler.UpdateMe)
			r.With(userLimiter).Put("/me", userHandler.UpdateMe)
			r.With(userLimiter).Post("/logout", userHandler.Logout)
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
			r.With(userLimiter).Delete("/metrics/{id}", metricsHandler.DeleteMetrics)
			r.With(userLimiter).Get("/metrics/stored", metricsHandler.GetStoredMetrics)
			r.With(userLimiter).Put("/metrics/{id}", metricsHandler.UpdateMetrics)

			r.With(userLimiter).Get("/daily-metrics", dailyMetricsHandler.GetDailyMetrics)
			r.With(userLimiter).Post("/daily-metrics", dailyMetricsHandler.CreateDailyMetric)
			r.With(userLimiter).Put("/daily-metrics/{id}", dailyMetricsHandler.UpdateDailyMetric)
			r.With(userLimiter).Get("/daily-metrics/{id}", dailyMetricsHandler.GetDailyMetricByID)
			r.With(userLimiter).Delete("/daily-metrics/{id}", dailyMetricsHandler.DeleteDailyMetric)
			r.With(userLimiter).Post("/daily-metrics/generate-advice", dailyMetricsHandler.GenerateAdvice)

			r.With(userLimiter).Post("/auth/passkeys/register/begin", passkeyHandler.BeginRegistration)
			r.With(userLimiter).Post("/auth/passkeys/register/finish", passkeyHandler.FinishRegistration)
			r.With(userLimiter).Get("/auth/passkeys", passkeyHandler.GetUserPasskeys)
			r.With(userLimiter).Delete("/auth/passkeys/{id}", passkeyHandler.DeletePasskey)
		})
	})

	// Frontend fallback for routes that are not API/static.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/static/") {
			log.Warn("not found", "path", req.URL.Path)
			http.NotFound(w, req)
			return
		}
		serveFrontendIndex(w, req)
	})

	return r
}

func serveFrontendIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/index.html")
}

func serveFrontendDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/dashboard.html")
}

func serveFrontendWorkouts(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/workouts.html")
}

func serveFrontendMetricsAnalytics(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/metrics_analytics.html")
}

func serveFrontendDailyMetrics(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/daily_metrics.html")
}

func serveFrontendStoredMetrics(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/metrics_stored.html")
}

func serveFrontendWorkoutDetails(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/workouts.html")
}

func serveFrontendLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/login.html")
}

func serveFrontendSignup(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/signup.html")
}

func serveFrontendProfile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "ui/templates/profile.html")
}
