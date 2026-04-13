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

func (r *passwordResetRepo) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *passwordResetRepo) CreateResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	db := r.getDB(ctx)

	query := `
        INSERT INTO password_resets (user_id, token_hash, expires_at)
        VALUES ($1, $2, $3)
    `
	_, err := db.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

func (r *passwordResetRepo) FindResetByTokenHash(ctx context.Context, tokenHash string) (userID int64, used bool, err error) {
	db := r.getDB(ctx)

	query := `
        SELECT user_id, used 
        FROM password_resets 
        WHERE token_hash = $1 
        AND expires_at > NOW()
    `
	err = db.QueryRow(ctx, query, tokenHash).Scan(&userID, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, my_errors.ErrTokenNotFound
	}
	return userID, used, err
}

func (r *passwordResetRepo) MarkAsUsed(ctx context.Context, tokenHash string) error {
	db := r.getDB(ctx)

	query := `
        UPDATE password_resets
		SET used = $1
		WHERE token_hash = $2
    `

	tag, err := db.Exec(ctx, query, true, tokenHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return my_errors.ErrPasswordResetHashNotFount
	}
	return nil
}

// ConsumeResetToken атомарно проверяет и помечает токен как использованный.
//
// КРИТИЧНО для защиты от race condition при двойном использовании токена:
// - Проверяет: токен существует, ещё не использован (used=false), не истёк
// - Проверяет: принадлежит указанному пользователю (user_id)
// - ПОМЕЧАЕТ как использованный (used=true) в ОДНОЙ SQL операции
// - Если условие не выполнено → ничего не меняет, возвращает ErrInvalidToken
//
// Если два запроса пытаются сбросить пароль с одним токеном:
//   - Первый UPDATE: used=false → меняет на true ✓
//   - Второй UPDATE: used=false → 0 строк! (уже true)
//     Возвращает ErrInvalidToken → ошибка
//
// Используется только в: PerformPasswordReset
func (r *passwordResetRepo) ConsumeResetToken(ctx context.Context, tokenHash string, userID int64) error {
	db := r.getDB(ctx)

	// Атомарная проверка + пометка в одной SQL операции
	query := `
        UPDATE password_resets
        SET used = true
        WHERE token_hash = $1
          AND user_id = $2
          AND used = false
          AND expires_at > NOW()
    `

	tag, err := db.Exec(ctx, query, tokenHash, userID)
	if err != nil {
		return err
	}

	// Если 0 строк обновлено, значит:
	// - токен не существует, ИЛИ
	// - уже использован (used=true), ИЛИ
	// - истёк срок, ИЛИ
	// - не принадлежит этому пользователю
	if tag.RowsAffected() == 0 {
		return my_errors.ErrInvalidToken
	}

	return nil
}

func (r *passwordResetRepo) DeleteResetToken(ctx context.Context, tokenHash string) error {
	db := r.getDB(ctx)

	query := `
	DELETE FROM password_resets WHERE token_hash = $1
	`
	tag, err := db.Exec(ctx, query, tokenHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return my_errors.ErrPasswordResetHashNotFount
	}
	return nil

}

func (r *passwordResetRepo) CleanupExpiredResetTokens(ctx context.Context) error {
	db := r.getDB(ctx)

	query := `DELETE FROM password_resets WHERE expires_at < NOW()`
	_, err := db.Exec(ctx, query)
	return err
}
