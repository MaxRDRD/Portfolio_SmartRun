package repository

import (
	"SmartRun/internal/model"
	"context"
)

type PasskeyRepository interface {
	Create(ctx context.Context, passkey *model.Passkey) error
	GetByID(ctx context.Context, id int64) (*model.Passkey, error)
	GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error)
	GetByUserID(ctx context.Context, userID int64) ([]model.Passkey, error)
	UpdateSignCountAndLastUsed(ctx context.Context, id int64, signCount uint32) error
	Delete(ctx context.Context, id int64, userID int64) error
}
