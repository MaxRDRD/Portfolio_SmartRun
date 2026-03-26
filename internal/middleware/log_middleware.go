package middleware

import (
	"SmartRun/internal/logger"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

func LogMiddleware(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Генерируем request_id для трейсинга
			requestID := uuid.New().String()

			// Создаём новый логгер с дополнительными полями
			requestLogger := baseLogger.With(
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)

			ctx := logger.WithContext(r.Context(), requestLogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
