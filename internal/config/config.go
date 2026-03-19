package config

import (
	"time"
)

type AuthConfig struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	JWTSecret       string
	PublicURL       string
	Email           EmailConfig
}

type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	UseTLS   bool // true для 465/587 с STARTTLS или implicit TLS
}
