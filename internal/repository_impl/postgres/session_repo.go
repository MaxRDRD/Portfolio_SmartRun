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

func (r *sessionRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *sessionRepository) DeleteSessionByHash(ctx context.Context, hash string) error {
	db := r.getDB(ctx)

	query := `
        DELETE FROM sessions
		WHERE refresh_hash = $1;
    `

	tag, err := db.Exec(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrTokenNotFound
	}

	return nil
}

// ConsumeSessionByHash атомарно удаляет валидную сессию и возвращает user_id.
// 
// КРИТИЧНО для защиты от race condition при одновременных refresh запросах:
// - Проверяет: токен существует, не отозван, не истёк
// - Удаляет токен в одной SQL операции
// - Возвращает user_id только если успешно удалил
// - Если токен уже удалён другим запросом → ErrTokenNotFound
//
// PostgreSQL гарантирует, что DELETE вернёт МАКСИМУМ 1 строку.
// Если две сессии пытаются ротировать одновременно:
//   - Первый DELETE успешен → получает user_id
//   - Второй DELETE: 0 строк затронуто → ErrTokenNotFound (из pgx.ErrNoRows)
//
// Используется только в: Refresh (с ротацией токена)
func (r *sessionRepository) ConsumeSessionByHash(ctx context.Context, hash string) (int64, error) {
	db := r.getDB(ctx)

	// Одна атомарная операция: найти + проверить + удалить + вернуть ID
	query := `
        DELETE FROM sessions
        WHERE refresh_hash = $1
          AND expires_at > NOW()
          AND revoked = false
        RETURNING user_id
    `

	var userID int64
	err := db.QueryRow(ctx, query, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Нет такого токена, или он уже истёк, или отозван, или удалён другим запросом
			return 0, my_errors.ErrTokenNotFound
		}
		return 0, fmt.Errorf("failed to consume session: %w", err)
	}

	return userID, nil
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
	db := r.getDB(ctx)

	query := `
	INSERT INTO sessions (user_id, refresh_hash, expires_at, revoked, created_at)
        VALUES ($1, $2, $3, $4, DEFAULT)
        RETURNING id, created_at
	`

	return db.QueryRow(ctx, query, session.UserId, session.RefreshHash, session.ExpiresAt, session.Revoked).Scan(&session.ID, &session.CreatedAt)

}

func (r *sessionRepository) FindSessionByHash(ctx context.Context, hash string) (*model.Session, error) {
	db := r.getDB(ctx)

	query := `
	SELECT id, user_id, expires_at, revoked, created_at FROM sessions WHERE refresh_hash = $1;
	`
	var session model.Session
	err := db.QueryRow(ctx, query, hash).Scan(
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
	db := r.getDB(ctx)

	query := `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'`
	_, err := db.Exec(ctx, query)
	return err
}

func (r *sessionRepository) DeleteAllSessionsForUser(ctx context.Context, userID int64) error {
	db := r.getDB(ctx)

	_, err := db.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	return err
}
