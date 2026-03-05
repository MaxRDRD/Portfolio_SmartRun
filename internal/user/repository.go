package user

import (
	mycrypto "SmartRun/internal/my_crypto"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	DeleteSessionByHash(ctx context.Context, refresh_hash string) error
	CreateSession(ctx context.Context, session *Session) error
	FindSessionByHash(ctx context.Context, hash string) (*Session, error)
	CreateTx(ctx context.Context, tx pgx.Tx, user *User) error
	CreateSessionTx(ctx context.Context, tx pgx.Tx, session *Session) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CleanupExpiredSessions(ctx context.Context) error
	GetTOTPSecret(ctx context.Context, userID int64) (string, error)
	UpdateTOTPSecret(ctx context.Context, userID int64, secret string, enabled bool) error
	IsTOTPEnabled(ctx context.Context, userID int64) (bool, error)
	DeleteAllSessionsForUser(ctx context.Context, userID int64) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, user *User) error {
	query := `
        INSERT INTO users (name, email, password, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING id, created_at
    `
	return r.db.QueryRow(ctx, query,
		user.Name,
		user.Email,
		user.Password,
	).Scan(&user.ID, &user.CreatedAt)
}

/*
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Name      string    `json:"name" validate:"required,min=4"`
	Email     string    `json:"email" validate:"required,email"`
	Password  string    `json:"pass" validate:"required,min=8"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
*/

func (r *repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
	SELECT id, name, email, password, created_at
	FROM users
	WHERE email = $1;
	`

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	return &user, err
}

func (r *repository) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `
	SELECT id, name, email, password, created_at FROM users WHERE id = $1;
	`

	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	return &user, err
}

func (r *repository) DeleteSessionByHash(ctx context.Context, hash string) error {
	query := `
        DELETE FROM sessions
		WHERE refresh_hash = $1;
    `

	tag, err := r.db.Exec(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrTokenNotFound
	}

	return nil
}

/*
id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT        NOT NULL REFERENCES users ON DELETE CASCADE,
    refresh_hash VARCHAR(128)  NOT NULL,
    expires_at  TIMESTAMPTZ   NOT NULL,
    revoked     BOOLEAN       DEFAULT FALSE,
    created_at  TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE (user_id, refresh_hash)
*/

func (r *repository) CreateSession(ctx context.Context, session *Session) error {
	query := `
	INSERT INTO sessions (user_id, refresh_hash, expires_at, revoked, created_at)
        VALUES ($1, $2, $3, $4, DEFAULT)
        RETURNING id, created_at
	`

	return r.db.QueryRow(ctx, query, session.UserId, session.RefreshHash, session.ExpiresAt, session.Revoked).Scan(&session.ID, &session.CreatedAt)

}

func (r *repository) FindSessionByHash(ctx context.Context, hash string) (*Session, error) {
	query := `
	SELECT id, user_id, expires_at, revoked, created_at FROM sessions WHERE refresh_hash = $1;
	`
	var session Session
	err := r.db.QueryRow(ctx, query, hash).Scan(
		&session.ID,
		&session.UserId,
		&session.ExpiresAt,
		&session.Revoked,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	if session.Revoked || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrInvalidToken
	}

	return &session, err
}

// CreateTx — версия Create с транзакцией
func (r *repository) CreateTx(ctx context.Context, tx pgx.Tx, user *User) error {
	query := `
        INSERT INTO users (name, email, password, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING id, created_at
    `
	return tx.QueryRow(ctx, query, user.Name, user.Email, user.Password).Scan(&user.ID, &user.CreatedAt)
}

// CreateSessionTx — версия CreateSession с транзакцией
func (r *repository) CreateSessionTx(ctx context.Context, tx pgx.Tx, session *Session) error {
	query := `
        INSERT INTO sessions (user_id, refresh_hash, expires_at, revoked, created_at)
        VALUES ($1, $2, $3, $4, DEFAULT)
        RETURNING id, created_at
    `
	return tx.QueryRow(ctx, query, session.UserId, session.RefreshHash, session.ExpiresAt, session.Revoked).Scan(&session.ID, &session.CreatedAt)
}

func (r *repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func (r *repository) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *repository) GetTOTPSecret(ctx context.Context, userID int64) (string, error) {
	query := `SELECT totp_secret FROM users WHERE id = $1;`

	var TOTPSecret string
	err := r.db.QueryRow(ctx, query, userID).Scan(&TOTPSecret)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}

	// Дешифруем
	plaintextSecret, err := mycrypto.Decrypt(TOTPSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}

	return plaintextSecret, err
}

func (r *repository) UpdateTOTPSecret(ctx context.Context, userID int64, secret string, enabled bool) error {
	encrypted, err := mycrypto.Encrypt(secret)
	if err != nil {
		return fmt.Errorf("failed to encrypt TOTP secret: %w", err)
	}

	query := `
	UPDATE users
	SET totp_secret=$1, totp_enabled=$2
	WHERE id=$3
	`

	tag, err := r.db.Exec(ctx, query, encrypted, enabled, userID)

	if err != nil {
		return fmt.Errorf("failed to update TOTPSecret: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrSecretNotFound
	}

	return nil
}

func (r *repository) IsTOTPEnabled(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT totp_enabled FROM users WHERE id = $1;`

	var TOTPEnabled bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&TOTPEnabled)

	return TOTPEnabled, err
}

func (r *repository) DeleteAllSessionsForUser(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}
