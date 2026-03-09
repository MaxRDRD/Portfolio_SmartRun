package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type TxManager interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
