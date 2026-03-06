package auth

import (
	"crypto/sha512"
	"encoding/hex"
)

func HashToken(token string) string {
	hash := sha512.Sum512([]byte(token))
	return hex.EncodeToString(hash[:])
}
