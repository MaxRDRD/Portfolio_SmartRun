package postgres

import (
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type DailyMetricRepo struct {
	db repository.DB
}

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
	body_battery_avg, total_calories, sleep_score, stress_avg, recommendation, streak_days, monotony, strain, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, now())
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
		dailyMetric.StressAvg,
		dailyMetric.Recommendation,
		dailyMetric.StreakDays,
		dailyMetric.Monotony,
		dailyMetric.Strain,
	).Scan(&dailyMetric.ID)

	if err != nil {
		log.Error("daily metrics repo: create failed", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "error", err)
		return nil, err
	}

	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	UPDATE daily_metrics
	SET date = $1, steps = $2, ctl = $3, atl = $4, tsb = $5, fatigue_score = $6,
	readiness_score = $7, body_battery_avg = $8, total_calories = $9, sleep_score = $10,
	stress_avg = $11, recommendation = $12, streak_days = $13, monotony = $14, strain = $15, updated_at = now()
	WHERE id = $16
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
		dailyMetric.StressAvg,
		dailyMetric.Recommendation,
		dailyMetric.StreakDays,
		dailyMetric.Monotony,
		dailyMetric.Strain,
		dailyMetric.ID,
	)
	if err != nil {
		log.Error("daily metrics repo: update failed", "id", dailyMetric.ID, "error", err)
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		log.Warn("daily metrics repo: update no rows affected", "id", dailyMetric.ID)
		return nil, my_errors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Delete(ctx context.Context, id int) error {
	log := logger.FromContext(ctx)
	sql := `DELETE FROM daily_metrics WHERE id = $1`
	db := r.getDB(ctx)
	tag, err := db.Exec(ctx, sql, id)
	if err != nil {
		log.Error("daily metrics repo: delete failed", "id", id, "error", err)
		return my_errors.ErrDailyMetricNotFound
	}
	if tag.RowsAffected() == 0 {
		log.Warn("daily metrics repo: delete no rows", "id", id)
		return my_errors.ErrDailyMetricNotFound
	}
	return nil
}

func (r *DailyMetricRepo) GetByID(ctx context.Context, id int) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	SELECT id, user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, stress_avg, recommendation, updated_at, streak_days, monotony, strain
	FROM daily_metrics
	WHERE id = $1
	`
	db := r.getDB(ctx)
	var dailyMetric model.DailyMetric
	err := db.QueryRow(ctx, sql, id).Scan(
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
		&dailyMetric.StressAvg,
		&dailyMetric.Recommendation,
		&dailyMetric.UpdatedAt,
		&dailyMetric.StreakDays,
		&dailyMetric.Monotony,
		&dailyMetric.Strain,
	)
	if err != nil {
		log.Error("daily metrics repo: get-by-id failed", "id", id, "error", err)
		return nil, my_errors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) GetAllByUserID(ctx context.Context, userId int64) ([]model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	sql := `
	SELECT id, user_id, date, steps, ctl, atl, tsb, fatigue_score, readiness_score,
	body_battery_avg, total_calories, sleep_score, stress_avg, recommendation, updated_at, streak_days, monotony, strain
	FROM daily_metrics
	WHERE user_id = $1
	ORDER BY date DESC
	`
	db := r.getDB(ctx)
	var dailyMetrics []model.DailyMetric
	rows, err := db.Query(ctx, sql, userId)
	if err != nil {
		log.Error("daily metrics repo: get-all query failed", "user_id", userId, "error", err)
		return nil, my_errors.ErrDailyMetricNotFound
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
			&dailyMetric.StressAvg,
			&dailyMetric.Recommendation,
			&dailyMetric.UpdatedAt,
			&dailyMetric.StreakDays,
			&dailyMetric.Monotony,
			&dailyMetric.Strain,
		)
		if err != nil {
			log.Error("daily metrics repo: get-all scan failed", "user_id", userId, "error", err)
			return nil, my_errors.ErrDailyMetricNotFound
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
	body_battery_avg, total_calories, sleep_score, stress_avg, recommendation, updated_at, streak_days, monotony, strain
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
		return nil, err
	}

	return &dailyMetric, nil
}

// UpdateOrCreate обновляет existing daily metric или создает новый (атомарное upsert)
func (r *DailyMetricRepo) UpdateOrCreate(ctx context.Context, dailyMetric *model.DailyMetric) error {
	// Сначала проверяем, существует ли запись
	existing, err := r.GetByUserIDAndDate(ctx, dailyMetric.UserID, dailyMetric.Date)
	if err != nil {
		return err
	}

	if existing != nil {
		// Обновляем существующую запись (сохраняем пользовательский ввод, обновляем расчёты)
		dailyMetric.ID = existing.ID
		
		// Сохраняем пользовательский ввод если его нет в новой метрике
		if dailyMetric.SleepScore == 0 && existing.SleepScore > 0 {
			dailyMetric.SleepScore = existing.SleepScore
		}
		if dailyMetric.StressAvg == 0 && existing.StressAvg > 0 {
			dailyMetric.StressAvg = existing.StressAvg
		}
		if dailyMetric.BodyBatteryAvg == 0 && existing.BodyBatteryAvg > 0 {
			dailyMetric.BodyBatteryAvg = existing.BodyBatteryAvg
		}
		
		_, err := r.Update(ctx, *dailyMetric)
		return err
	}

	// Создаём новую запись
	_, err = r.Create(ctx, *dailyMetric)
	return err
}
