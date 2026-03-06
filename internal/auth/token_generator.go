package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 64) // 64 байта = 512 бит
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("ошибка генерации байтов: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateAccessTokenTyped — вариант с явной структурой claims
func GenerateAccessToken(userID int64, duration time.Duration) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not set")
	}

	claims := AccessTokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			// Subject:   fmt.Sprintf("%d", userID), // можно и здесь, если хочешь
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}
