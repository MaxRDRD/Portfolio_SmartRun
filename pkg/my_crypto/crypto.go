package mycrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrInvalidKeyLength = errors.New("master key must be 32 bytes")
	ErrDecryptionFailed = errors.New("decryption failed - wrong key or corrupted data")
)

func getMasterKey() ([]byte, error) {
	keyBase64 := os.Getenv("TOTP_MASTER_KEY")
	if keyBase64 == "" {
		return nil, errors.New("TOTP_MASTER_KEY not set in environment")
	}

	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid TOTP_MASTER_KEY format: %w", err)
	}

	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	return key, nil
}

// Encrypt возвращает base64-строку: nonce.ciphertext.tag
func Encrypt(plaintext string) (string, error) {
	key, err := getMasterKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Формат: nonce || ciphertext || tag (tag уже внутри Seal)
	combined := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt принимает base64-строку в формате nonce.ciphertext.tag
func Decrypt(encryptedBase64 string) (string, error) {
	key, err := getMasterKey()
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	if len(data) < 12 { // минимальный nonce для GCM — 12 байт
		return "", errors.New("encrypted data too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("invalid encrypted data")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed // обычно — неверный ключ или подделка
	}

	return string(plaintext), nil
}
