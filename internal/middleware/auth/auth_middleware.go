package my_middleware

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/logger"
	"SmartRun/pkg/response"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	secret string
}

func NewAuthMiddleware(secret string) *AuthMiddleware {
	return &AuthMiddleware{secret: secret}
}

func (m *AuthMiddleware) JWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			response.SendUnauthorized(w, "missing authorization header")
			return
		}

		if !strings.HasPrefix(tokenString, "Bearer ") {
			response.SendUnauthorized(w, "invalid authorization header")
			return
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		claims := &auth.AccessTokenClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 { // Уточняем: только HS256
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(m.secret), nil
			})

		if err != nil || !token.Valid {
			response.SendUnauthorized(w, "invalid token")
			return
		}

		reqLogger := logger.FromContext(r.Context()).With("user_id", claims.UserID)
		ctx := logger.WithContext(r.Context(), reqLogger)
		ctx = context.WithValue(ctx, auth.UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
