package repository

import (
	"SmartRun/internal/model"
	"context"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	GetEmailByID(ctx context.Context, id int64) (string, error)
	UpdatePassword(ctx context.Context, userID int64, newHash string) error
}
