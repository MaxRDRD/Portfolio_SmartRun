package user

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=4"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_access_token"`
	Require2FA  bool   `json:"require_2fa,omitempty"`
	Message     string `json:"message"`
}

type UserResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

type Session struct {
	ID          uuid.UUID `json:"id"`
	UserId      int64     `json:"user_id"`
	RefreshHash string    `json:"refresh_hash"`
	ExpiresAt   time.Time `json:"expires_at"`
	Revoked     bool      `json:"revoked"`
	CreatedAt   time.Time `json:"ceated_At"`
}

// AccessTokenClaims — структура claims для access-токена
type AccessTokenClaims struct {
	UserID int64 `json:"sub"`
	jwt.RegisteredClaims
}

type Enable2FAResponse struct {
	QRBase64 string `json:"qr_base64,omitempty"` // или отдельно возвращаем image
	Secret   string `json:"secret,omitempty"`    // только для ручной настройки, один раз
}

type AuthResult struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int           `json:"expires_in"`
	User         *UserResponse `json:"user"`
}
