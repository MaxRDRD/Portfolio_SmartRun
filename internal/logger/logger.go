package logger

import (
	"SmartRun/internal/config"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type key string

const loggerKey key = "logger"

func NewLogger(cfg config.LoggerConfig) (*slog.Logger, func() error) {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
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

	var writer io.Writer
	var closer func() error = func() error { return nil }

	switch strings.ToLower(cfg.Output) {
	case "file":
		f := getLogFile(cfg.FilePath)
		writer = f
		if f != os.Stdout {
			closer = f.Close
		}
	case "both":
		f := getLogFile(cfg.FilePath)
		writer = io.MultiWriter(os.Stdout, f)
		if f != os.Stdout {
			closer = f.Close
		}
	case "stdout", "":
		writer = os.Stdout
	default:
		writer = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(writer, opts)
	return slog.New(handler), closer
}

func getLogFile(filePath string) *os.File {
	if filePath == "" {
		filePath = "logs/app.log"
	}
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return os.Stdout
		}
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
