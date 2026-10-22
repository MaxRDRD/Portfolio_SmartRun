package postgres

import (
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/apperrors"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DailyMetricRepo struct {
	db repository.DB
}

var _ repository.DailyMetricRepository = (*DailyMetricRepo)(nil)

func NewDailyMetricRepository(db repository.DB) repository.DailyMetricRepository {
	return &DailyMetricRepo{db: db}
}

func (r *DailyMetricRepo) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *DailyMetricRepo) Create(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	INSERT INTO daily_metrics (user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, sleep_hours, stress_avg, recommendation, streak_days, monotony, strain, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
	RETURNING id
	`
	db := r.getDB(ctx)
	err := db.QueryRow(ctx, sql,
		dailyMetric.UserID,
		dailyMetric.Date,
		dailyMetric.Steps,
		dailyMetric.CTL,
		dailyMetric.ATL,
		dailyMetric.TSB,
		dailyMetric.FatigueScore,
		dailyMetric.ReadinessScore,
		dailyMetric.BodyBatteryAvg,
		dailyMetric.TotalCalories,
		dailyMetric.SleepScore,
		dailyMetric.SleepHours,
		dailyMetric.StressAvg,
		dailyMetric.Recommendation,
		dailyMetric.StreakDays,
		dailyMetric.Monotony,
		dailyMetric.Strain,
	).Scan(&dailyMetric.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Warn("daily metrics repo: create duplicate", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "constraint", pgErr.ConstraintName)
			return nil, apperrors.ErrDailyMetricAlreadyExists
		}
		log.Error("daily metrics repo: create failed", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "error", err)
		return nil, fmt.Errorf("Create: %w", err)
	}

	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	UPDATE daily_metrics
	SET date = $1, steps = $2, ctl = $3, atl = $4, tsb = $5, fatigue_score = $6,
	readiness_score = $7, body_battery_avg = $8, total_calories = $9, sleep_score = $10,
	sleep_hours = $11, stress_avg = $12, recommendation = $13, streak_days = $14, monotony = $15, strain = $16, updated_at = now()
	WHERE id = $17 AND user_id = $18
	`

	db := r.getDB(ctx)
	tag, err := db.Exec(ctx, sql,
		dailyMetric.Date,
		dailyMetric.Steps,
		dailyMetric.CTL,
		dailyMetric.ATL,
		dailyMetric.TSB,
		dailyMetric.FatigueScore,
		dailyMetric.ReadinessScore,
		dailyMetric.BodyBatteryAvg,
		dailyMetric.TotalCalories,
		dailyMetric.SleepScore,
		dailyMetric.SleepHours,
		dailyMetric.StressAvg,
		dailyMetric.Recommendation,
		dailyMetric.StreakDays,
		dailyMetric.Monotony,
		dailyMetric.Strain,
		dailyMetric.ID,
		dailyMetric.UserID,
	)
	if err != nil {
		log.Error("daily metrics repo: update failed", "id", dailyMetric.ID, "error", err)
		return nil, fmt.Errorf("Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		log.Warn("daily metrics repo: update no rows affected", "id", dailyMetric.ID)
		return nil, apperrors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Delete(ctx context.Context, id, userID int64) error {
	log := logger.FromContext(ctx)
	sql := `DELETE FROM daily_metrics WHERE id = $1 and user_id = $2`
	db := r.getDB(ctx)
	tag, err := db.Exec(ctx, sql, id, userID)
	if err != nil {
		log.Error("daily metrics repo: delete failed", "id", id, "error", err)
		return apperrors.ErrDailyMetricNotFound
	}
	if tag.RowsAffected() == 0 {
		log.Warn("daily metrics repo: delete no rows", "id", id)
		return apperrors.ErrDailyMetricNotFound
	}
	return nil
}

func (r *DailyMetricRepo) GetByID(ctx context.Context, id, userID int64) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	SELECT id, user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, sleep_hours, stress_avg, recommendation, updated_at, streak_days, monotony, strain
	FROM daily_metrics
	WHERE id = $1 and user_id = $2
	`
	db := r.getDB(ctx)
	var dailyMetric model.DailyMetric
	err := db.QueryRow(ctx, sql, id, userID).Scan(
		&dailyMetric.ID,
		&dailyMetric.UserID,
		&dailyMetric.Date,
		&dailyMetric.Steps,
		&dailyMetric.CTL,
		&dailyMetric.ATL,
		&dailyMetric.TSB,
		&dailyMetric.FatigueScore,
		&dailyMetric.ReadinessScore,
		&dailyMetric.BodyBatteryAvg,
		&dailyMetric.TotalCalories,
		&dailyMetric.SleepScore,
		&dailyMetric.SleepHours,
		&dailyMetric.StressAvg,
		&dailyMetric.Recommendation,
		&dailyMetric.UpdatedAt,
		&dailyMetric.StreakDays,
		&dailyMetric.Monotony,
		&dailyMetric.Strain,
	)
	if err != nil {
		log.Error("daily metrics repo: get-by-id failed", "id", id, "error", err)
		return nil, apperrors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) GetAllByUserID(ctx context.Context, userId int64) ([]model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	SELECT id, user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, sleep_hours, stress_avg, recommendation, updated_at, streak_days, monotony, strain
	FROM daily_metrics
	WHERE user_id = $1
	ORDER BY date DESC
	`
	db := r.getDB(ctx)
	var dailyMetrics []model.DailyMetric
	rows, err := db.Query(ctx, sql, userId)
	if err != nil {
		log.Error("daily metrics repo: get-all query failed", "user_id", userId, "error", err)
		return nil, apperrors.ErrDailyMetricNotFound
	}
	defer rows.Close()
	for rows.Next() {
		var dailyMetric model.DailyMetric
		err := rows.Scan(
			&dailyMetric.ID,
			&dailyMetric.UserID,
			&dailyMetric.Date,
			&dailyMetric.Steps,
			&dailyMetric.CTL,
			&dailyMetric.ATL,
			&dailyMetric.TSB,
			&dailyMetric.FatigueScore,
			&dailyMetric.ReadinessScore,
			&dailyMetric.BodyBatteryAvg,
			&dailyMetric.TotalCalories,
			&dailyMetric.SleepScore,
			&dailyMetric.SleepHours,
			&dailyMetric.StressAvg,
			&dailyMetric.Recommendation,
			&dailyMetric.UpdatedAt,
			&dailyMetric.StreakDays,
			&dailyMetric.Monotony,
			&dailyMetric.Strain,
		)
		if err != nil {
			log.Error("daily metrics repo: get-all scan failed", "user_id", userId, "error", err)
			return nil, apperrors.ErrDailyMetricNotFound
		}
		dailyMetrics = append(dailyMetrics, dailyMetric)
	}
	return dailyMetrics, nil
}

// GetByUserIDAndDate получает daily metric по пользователю и дате
func (r *DailyMetricRepo) GetByUserIDAndDate(ctx context.Context, userID int64, date time.Time) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	SELECT id, user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, sleep_hours, stress_avg, recommendation, updated_at, streak_days, monotony, strain
	FROM daily_metrics
	WHERE user_id = $1 AND DATE(date) = DATE($2)
	LIMIT 1
	`
	db := r.getDB(ctx)
	var dailyMetric model.DailyMetric
	err := db.QueryRow(ctx, sql, userID, date).Scan(
		&dailyMetric.ID,
		&dailyMetric.UserID,
		&dailyMetric.Date,
		&dailyMetric.Steps,
		&dailyMetric.CTL,
		&dailyMetric.ATL,
		&dailyMetric.TSB,
		&dailyMetric.FatigueScore,
		&dailyMetric.ReadinessScore,
		&dailyMetric.BodyBatteryAvg,
		&dailyMetric.TotalCalories,
		&dailyMetric.SleepScore,
		&dailyMetric.SleepHours,
		&dailyMetric.StressAvg,
		&dailyMetric.Recommendation,
		&dailyMetric.UpdatedAt,
		&dailyMetric.StreakDays,
		&dailyMetric.Monotony,
		&dailyMetric.Strain,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// нет записи для этого дня - нормально, вернуть nil (не ошибка)
			return nil, nil
		}
		// Любая другая SQL ошибка должна пробрасываться наверх,
		// иначе транзакция останется aborted и упадет следующей командой с SQLSTATE 25P02.
		log.Error("daily metrics repo: get-by-date failed", "user_id", userID, "date", date.Format("2006-01-02"), "error", err)
		return nil, fmt.Errorf("GetByUserIDAndDate: %w", err)
	}

	return &dailyMetric, nil
}

// UpdateOrCreate обновляет existing daily metric или создает новый (атомарное upsert)
func (r *DailyMetricRepo) UpdateOrCreate(ctx context.Context, dailyMetric *model.DailyMetric) error {
	log := logger.FromContext(ctx)
	sql := `
	INSERT INTO daily_metrics (user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
    body_battery_avg, total_calories, sleep_score, sleep_hours, stress_avg, recommendation, streak_days, monotony, strain, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
ON CONFLICT (user_id, date) DO UPDATE SET
    steps            = EXCLUDED.steps,
    ctl              = EXCLUDED.ctl,
    atl              = EXCLUDED.atl,
    tsb              = EXCLUDED.tsb,
    fatigue_score    = EXCLUDED.fatigue_score,
    readiness_score  = EXCLUDED.readiness_score,
    body_battery_avg = COALESCE(NULLIF(EXCLUDED.body_battery_avg, 0), daily_metrics.body_battery_avg),
    total_calories   = EXCLUDED.total_calories,
    sleep_score      = COALESCE(NULLIF(EXCLUDED.sleep_score, 0), daily_metrics.sleep_score),
    sleep_hours      = COALESCE(NULLIF(EXCLUDED.sleep_hours, 0), daily_metrics.sleep_hours),
    stress_avg       = COALESCE(NULLIF(EXCLUDED.stress_avg, 0), daily_metrics.stress_avg),
    recommendation   = COALESCE(NULLIF(EXCLUDED.recommendation, ''), daily_metrics.recommendation),
    streak_days      = EXCLUDED.streak_days,
    monotony         = EXCLUDED.monotony,
    strain           = EXCLUDED.strain,
    updated_at       = now()
RETURNING id;
	`

	db := r.getDB(ctx)
	err := db.QueryRow(ctx, sql,
		dailyMetric.UserID,
		dailyMetric.Date,
		dailyMetric.Steps,
		dailyMetric.CTL,
		dailyMetric.ATL,
		dailyMetric.TSB,
		dailyMetric.FatigueScore,
		dailyMetric.ReadinessScore,
		dailyMetric.BodyBatteryAvg,
		dailyMetric.TotalCalories,
		dailyMetric.SleepScore,
		dailyMetric.SleepHours,
		dailyMetric.StressAvg,
		dailyMetric.Recommendation,
		dailyMetric.StreakDays,
		dailyMetric.Monotony,
		dailyMetric.Strain,
	).Scan(&dailyMetric.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Warn("daily metrics repo: create duplicate", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "constraint", pgErr.ConstraintName)
			return apperrors.ErrDailyMetricAlreadyExists
		}
		log.Error("daily metrics repo: create failed", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "error", err)
		return fmt.Errorf("Create: %w", err)
	}

	return nil
}
