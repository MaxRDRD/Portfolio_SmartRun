package ratelimit

import (
	"SmartRun/internal/auth"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

type RateLimitConfig struct {
	Requests int
	Window   time.Duration
	KeyBy    []string
}

// KeyFuncRegistry — простая фабрика именованных ключей
var KeyFuncRegistry = map[string]httprate.KeyFunc{
	"ip":       httprate.KeyByIP,
	"real_ip":  httprate.KeyByRealIP,
	"endpoint": httprate.KeyByEndpoint,
	"user_id":  KeyByUserID, // твоя кастомная
	// "api_key":  KeyByAPIKey,
	// "static:admin": httprate.Key("admin"), // статический ключ
}

// Пример фабрики middleware (можно принимать конфиг)
func NewRateLimitMiddleware(cfg RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.Requests <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	var keyFuncs []httprate.KeyFunc

	for _, name := range cfg.KeyBy {
		if fn, ok := KeyFuncRegistry[name]; ok {
			keyFuncs = append(keyFuncs, fn)
		}
		// если неизвестное имя — можно логгировать warning
	}

	opts := []httprate.Option{}
	if len(keyFuncs) > 0 {
		opts = append(opts, httprate.WithKeyFuncs(keyFuncs...))
	}

	limiter := httprate.NewRateLimiter(cfg.Requests, cfg.Window, opts...)

	return limiter.Handler
}

// KeyByUserID — ключ по user_id (после авторизации)
func KeyByUserID(r *http.Request) (string, error) {
	if id, ok := auth.GetUserID(r.Context()); ok {
		return fmt.Sprintf("user:%d", id), nil
	}
	return httprate.KeyByIP(r) // fallback на IP
}
