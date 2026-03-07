package repository

import (
	"context"
	"time"
)

type PasswordResetRepository interface {
	CreateResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	FindResetByTokenHash(ctx context.Context, tokenHash string) (int64, bool, error) // возвращает userID, used, error
	MarkAsUsed(ctx context.Context, tokenHash string) error
	DeleteResetToken(ctx context.Context, tokenHash string) error
	CleanupExpiredResetTokens(ctx context.Context) error
}
