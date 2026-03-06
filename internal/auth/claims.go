package auth

import "github.com/golang-jwt/jwt/v5"

type AccessTokenClaims struct {
	UserID int64 `json:"sub"`
	jwt.RegisteredClaims
}
