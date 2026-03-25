package postgres

import (
	"context"
	"errors"

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

func (t *txManager) WithTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return err
	}

	// Важно: defer с обработкой panic + err
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		var rbErr error
		if err != nil {
			rbErr = tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx) // присваиваем err, чтобы внешний вызов видел ошибку Commit
		}
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			// log.Warn("transaction finalization failed", "error", rbErr)
		}

		// Самое важное: НЕ перезаписываем err здесь!
		// err остаётся тем, что вернула fn (или panic)
	}()

	ctx = context.WithValue(ctx, txKey{}, tx)
	err = fn(ctx)
	return err
}

// Вспомогательная функция для репозиториев
func GetTx(ctx context.Context) (pgx.Tx, bool) {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx, true
	}
	return nil, false
}
