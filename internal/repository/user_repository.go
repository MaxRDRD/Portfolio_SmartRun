package repository

import (
	"SmartRun/internal/model"
	"context"

	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	GetEmailByID(ctx context.Context, id int64) (string, error)
	CreateTx(ctx context.Context, tx pgx.Tx, user *model.User) error
	UpdatePassword(ctx context.Context, userID int64, newHash string) error
}
