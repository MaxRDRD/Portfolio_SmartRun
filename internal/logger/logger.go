package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type key string

const loggerKey key = "logger"

func NewLogger() *slog.Logger {
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	multi := io.MultiWriter(os.Stdout, getLogFile())

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(multi, opts)
	return slog.New(handler)
}

func getLogFile() *os.File {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return os.Stdout
	}
	f, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return os.Stdout
	}
	return f
}

// WithContext добавляет логгер в контекст
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext достаёт логгер из контекста (с fallback)
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default() // fallback
}
