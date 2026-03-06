package model

import "time"

type AuthConfig struct {
	JWTSecret string
}

type Session struct {
	ID          int64     `json:"id"`
	UserId      int64     `json:"user_id"`
	RefreshHash string    `json:"refresh_hash"`
	ExpiresAt   time.Time `json:"expires_at"`
	Revoked     bool      `json:"revoked"`
	CreatedAt   time.Time `json:"ceated_At"`
}
