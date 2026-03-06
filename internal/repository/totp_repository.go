package repository

import (
	"context"
)

type TotpRepository interface {
	GetTOTPSecret(ctx context.Context, userID int64) (string, error)
	UpdateTOTPSecret(ctx context.Context, userID int64, secret string, enabled bool) error
	IsTOTPEnabled(ctx context.Context, userID int64) (bool, error)
}
