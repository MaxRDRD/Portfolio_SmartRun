package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SmartRun/internal/repository"
)

type txManager struct {
	db *pgxpool.Pool
}

func NewTxManager(db *pgxpool.Pool) repository.TxManager {
	return &txManager{db: db}
}

func (t *txManager) Begin(ctx context.Context) (pgx.Tx, error) {
	return t.db.Begin(ctx)
}

type txKey struct{}
