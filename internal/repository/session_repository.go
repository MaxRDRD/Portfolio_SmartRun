package repository

import (
	"SmartRun/internal/model"
	"context"
)

type SessionRepository interface {
	DeleteSessionByHash(ctx context.Context, refresh_hash string) error
	CreateSession(ctx context.Context, session *model.Session) error
	FindSessionByHash(ctx context.Context, hash string) (*model.Session, error)
	ConsumeSessionByHash(ctx context.Context, hash string) (int64, error) // atomically delete and return user_id
	CleanupExpiredSessions(ctx context.Context) error
	DeleteAllSessionsForUser(ctx context.Context, userID int64) error
}
