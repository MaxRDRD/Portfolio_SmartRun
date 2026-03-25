package postgres

import (
	"SmartRun/internal/repository"
	mycrypto "SmartRun/pkg/my_crypto"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type totpRepository struct {
	db repository.DB
}

func NewTOTPRepository(db repository.DB) repository.TotpRepository {
	return &totpRepository{db: db}
}

func (r *totpRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *totpRepository) GetTOTPSecret(ctx context.Context, userID int64) (string, error) {
	db := r.getDB(ctx)

	query := `SELECT totp_secret FROM users WHERE id = $1;`

	var TOTPSecret string
	err := db.QueryRow(ctx, query, userID).Scan(&TOTPSecret)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if TOTPSecret == "" {
		return "", nil
	}

	// Дешифруем
	plaintextSecret, err := mycrypto.Decrypt(TOTPSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}

	return plaintextSecret, err
}

func (r *totpRepository) UpdateTOTPSecret(ctx context.Context, userID int64, secret string, enabled bool) error {
	db := r.getDB(ctx)

	encrypted, err := mycrypto.Encrypt(secret)
	if err != nil {
		return fmt.Errorf("failed to encrypt TOTP secret: %w", err)
	}

	query := `
	UPDATE users
	SET totp_secret=$1, totp_enables=$2
	WHERE id=$3
	`

	tag, err := db.Exec(ctx, query, encrypted, enabled, userID)

	if err != nil {
		return fmt.Errorf("failed to update TOTPSecret: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrSecretNotFound
	}

	return nil
}

func (r *totpRepository) IsTOTPEnabled(ctx context.Context, userID int64) (bool, error) {
	db := r.getDB(ctx)

	query := `SELECT COALESCE(totp_enables, false) FROM users WHERE id = $1;`

	var TOTPEnabled bool
	err := db.QueryRow(ctx, query, userID).Scan(&TOTPEnabled)

	return TOTPEnabled, err
}
