package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/apperrors"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type passkeyRepository struct {
	db repository.DB
}

var _ repository.PasskeyRepository = (*passkeyRepository)(nil)

func NewPasskeyRepository(db repository.DB) repository.PasskeyRepository {
	return &passkeyRepository{db: db}
}

func (r *passkeyRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *passkeyRepository) Create(ctx context.Context, passkey *model.Passkey) error {
	db := r.getDB(ctx)

	query := `
		INSERT INTO user_passkeys (
			user_id, credential_id, public_key, attestation_type,
			sign_count, aaguid, transports, backup_state, device_name, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
		)
		RETURNING id, created_at
	`

	err := db.QueryRow(ctx, query,
		passkey.UserID,
		passkey.CredentialID,
		passkey.PublicKey,
		passkey.AttestationType,
		passkey.SignCount,
		passkey.AAGUID,
		passkey.Transports,
		passkey.BackupState,
		passkey.DeviceName,
	).Scan(&passkey.ID, &passkey.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.ErrPasskeyAlreadyExists
		}
		return fmt.Errorf("passkeyRepo.Create: %w", err)
	}

	return nil
}

func (r *passkeyRepository) GetByID(ctx context.Context, id int64) (*model.Passkey, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, user_id, credential_id, public_key, attestation_type,
		       aaguid, sign_count, transports, backup_state, device_name, created_at, last_used_at
		FROM user_passkeys
		WHERE id = $1
	`

	pk := &model.Passkey{}
	err := db.QueryRow(ctx, query, id).Scan(
		&pk.ID,
		&pk.UserID,
		&pk.CredentialID,
		&pk.PublicKey,
		&pk.AttestationType,
		&pk.AAGUID,
		&pk.SignCount,
		&pk.Transports,
		&pk.BackupState,
		&pk.DeviceName,
		&pk.CreatedAt,
		&pk.LastUsedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrPasskeyNotFound
		}
		return nil, fmt.Errorf("passkeyRepo.GetByID: %w", err)
	}

	return pk, nil
}

func (r *passkeyRepository) GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, user_id, credential_id, public_key, attestation_type,
		       aaguid, sign_count, transports, backup_state, device_name, created_at, last_used_at
		FROM user_passkeys
		WHERE credential_id = $1
	`

	pk := &model.Passkey{}
	err := db.QueryRow(ctx, query, credentialID).Scan(
		&pk.ID,
		&pk.UserID,
		&pk.CredentialID,
		&pk.PublicKey,
		&pk.AttestationType,
		&pk.AAGUID,
		&pk.SignCount,
		&pk.Transports,
		&pk.BackupState,
		&pk.DeviceName,
		&pk.CreatedAt,
		&pk.LastUsedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrPasskeyNotFound
		}
		return nil, fmt.Errorf("passkeyRepo.GetByCredentialID: %w", err)
	}

	return pk, nil
}

func (r *passkeyRepository) GetByUserID(ctx context.Context, userID int64) ([]model.Passkey, error) {
	db := r.getDB(ctx)

	query := `
		SELECT id, user_id, credential_id, public_key, attestation_type,
		       aaguid, sign_count, transports, backup_state, device_name, created_at, last_used_at
		FROM user_passkeys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("passkeyRepo.GetByUserID: %w", err)
	}
	defer rows.Close()

	var passkeys []model.Passkey
	for rows.Next() {
		var pk model.Passkey
		err := rows.Scan(
			&pk.ID,
			&pk.UserID,
			&pk.CredentialID,
			&pk.PublicKey,
			&pk.AttestationType,
			&pk.AAGUID,
			&pk.SignCount,
			&pk.Transports,
			&pk.BackupState,
			&pk.DeviceName,
			&pk.CreatedAt,
			&pk.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("passkeyRepo.GetByUserID scan: %w", err)
		}
		passkeys = append(passkeys, pk)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("passkeyRepo.GetByUserID rows: %w", err)
	}

	return passkeys, nil
}

func (r *passkeyRepository) UpdateSignCountAndLastUsed(ctx context.Context, id int64, signCount uint32) error {
	db := r.getDB(ctx)

	query := `
		UPDATE user_passkeys
		SET sign_count = $1, last_used_at = NOW()
		WHERE id = $2
	`

	cmdTag, err := db.Exec(ctx, query, signCount, id)
	if err != nil {
		return fmt.Errorf("passkeyRepo.UpdateSignCountAndLastUsed: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return apperrors.ErrPasskeyNotFound
	}

	return nil
}

func (r *passkeyRepository) Delete(ctx context.Context, id int64, userID int64) error {
	db := r.getDB(ctx)

	query := `DELETE FROM user_passkeys WHERE id = $1 AND user_id = $2`

	cmdTag, err := db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("passkeyRepo.Delete: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return apperrors.ErrPasskeyNotFound
	}

	return nil
}
