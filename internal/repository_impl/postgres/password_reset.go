package postgres

import (
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type passwordResetRepo struct {
	db repository.DB
}

func NewPasswordResetRepository(db repository.DB) repository.PasswordResetRepository {
	return &passwordResetRepo{db: db}
}

func (r *passwordResetRepo) CreateResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	query := `
        INSERT INTO password_resets (user_id, token_hash, expires_at)
        VALUES ($1, $2, $3)
    `
	_, err := r.db.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

func (r *passwordResetRepo) FindResetByTokenHash(ctx context.Context, tokenHash string) (userID int64, used bool, err error) {
	query := `
        SELECT user_id, used 
        FROM password_resets 
        WHERE token_hash = $1 
          AND expires_at > NOW()
    `
	err = r.db.QueryRow(ctx, query, tokenHash).Scan(&userID, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, my_errors.ErrTokenNotFound
	}
	return userID, used, err
}

func (r *passwordResetRepo) MarkAsUsed(ctx context.Context, tokenHash string) error {
	query := `
        UPDATE password_resets
		SET used = $1
		WHERE token_hash = $2
    `

	tag, err := r.db.Exec(ctx, query, true, tokenHash)

	if tag.RowsAffected() == 0 {
		return my_errors.ErrPasswordResetHashNotFount
	}
	if err != nil {
		return my_errors.ErrInvalidData
	}
	return nil
}

// DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'
func (r *passwordResetRepo) DeleteResetToken(ctx context.Context, tokenHash string) error {
	query := `
	DELETE FROM password_resets WHERE token_hash = $1
	`
	tag, err := r.db.Exec(ctx, query, tokenHash)

	if tag.RowsAffected() == 0 {
		return my_errors.ErrPasswordResetHashNotFount
	}
	if err != nil {
		return my_errors.ErrInvalidData
	}
	return nil

}

func (r *passwordResetRepo) CleanupExpiredResetTokens(ctx context.Context) error {
	query := `DELETE FROM password_resets WHERE expires_at < NOW() - INTERVAL '1 hour'`
	_, err := r.db.Exec(ctx, query)
	return err
}
