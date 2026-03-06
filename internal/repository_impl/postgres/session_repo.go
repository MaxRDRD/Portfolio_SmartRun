package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type sessionRepository struct {
	db repository.DB
}

func NewSessionRepository(db repository.DB) repository.SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) DeleteSessionByHash(ctx context.Context, hash string) error {
	query := `
        DELETE FROM sessions
		WHERE refresh_hash = $1;
    `

	tag, err := r.db.Exec(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrTokenNotFound
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

func (r *sessionRepository) CreateSession(ctx context.Context, session *model.Session) error {
	query := `
	INSERT INTO sessions (user_id, refresh_hash, expires_at, revoked, created_at)
        VALUES ($1, $2, $3, $4, DEFAULT)
        RETURNING id, created_at
	`

	return r.db.QueryRow(ctx, query, session.UserId, session.RefreshHash, session.ExpiresAt, session.Revoked).Scan(&session.ID, &session.CreatedAt)

}

func (r *sessionRepository) FindSessionByHash(ctx context.Context, hash string) (*model.Session, error) {
	query := `
	SELECT id, user_id, expires_at, revoked, created_at FROM sessions WHERE refresh_hash = $1;
	`
	var session model.Session
	err := r.db.QueryRow(ctx, query, hash).Scan(
		&session.ID,
		&session.UserId,
		&session.ExpiresAt,
		&session.Revoked,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, my_errors.ErrTokenNotFound
		}
		return nil, err
	}

	if session.Revoked || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, my_errors.ErrInvalidToken
	}

	return &session, err
}

func (r *sessionRepository) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *sessionRepository) DeleteAllSessionsForUser(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}
