package repository

import (
	"context"
	"time"
)

type PasswordResetRepository interface {
	CreateResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	FindResetByTokenHash(ctx context.Context, tokenHash string) (int64, bool, error) // возвращает userID, used, error
	MarkAsUsed(ctx context.Context, tokenHash string) error
	ConsumeResetToken(ctx context.Context, tokenHash string, userID int64) error // atomically verify, mark used, and validate
	DeleteResetToken(ctx context.Context, tokenHash string) error
	CleanupExpiredResetTokens(ctx context.Context) error
}
