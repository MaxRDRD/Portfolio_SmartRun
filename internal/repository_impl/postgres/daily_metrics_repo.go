package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
)

type DailyMetricRepo struct {
	db repository.DB
}

func NewDailyMetricRepository(db repository.DB) repository.DailyMetricRepository {
	return &DailyMetricRepo{db: db}
}

func (r *DailyMetricRepo) getDB(ctx context.Context) repository.DB {
	if tx, ok := getTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *DailyMetricRepo) Create(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	sql := `
	INSERT INTO daily_metrics (date, steps, ctl, atl, tsb, fatigure_score, readiness_score,
	body_baterry_avg, total_calories, sleep_score, stress_avg, reco,endation, update_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
	RETURNING id
	`
	db := r.getDB(ctx)
	err := db.QueryRow(ctx, sql,
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
	).Scan(&dailyMetric.ID)

	if err != nil {
		return nil, my_errors.ErrDailyMetricAlreadyExists
	}

	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	sql := `
	UPDATE daily_metrics
	SET date = $1, steps = $2, ctl = $3, atl = $4, tsb = $5, fatigure_score = $6,
	readiness_score = $7, body_baterry_avg = $8, total_calories = $9, sleep_score = $10,
	stress_avg = $11, reco,endation = $12, update_at = now()
	WHERE id = $13
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
		dailyMetric.ID,
	)
	if err != nil {
		return nil, my_errors.ErrDailyMetricNotFound
	}
	if tag.RowsAffected() == 0 {
		return nil, my_errors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) Delete(ctx context.Context, id int) error {
	sql := `DELETE FROM daily_metrics WHERE id = $1`
	db := r.getDB(ctx)
	tag, err := db.Exec(ctx, sql, id)
	if err != nil {
		return my_errors.ErrDailyMetricNotFound
	}
	if tag.RowsAffected() == 0 {
		return my_errors.ErrDailyMetricNotFound
	}
	return nil
}

func (r *DailyMetricRepo) GetByID(ctx context.Context, id int) (*model.DailyMetric, error) {
	sql := `
	SELECT id, date, steps, ctl, atl, tsb, fatigure_score, readiness_score,
	body_baterry_avg, total_calories, sleep_score, stress_avg, reco,endation, update_at
	FROM daily_metrics
	WHERE id = $1
	`
	db := r.getDB(ctx)
	var dailyMetric model.DailyMetric
	err := db.QueryRow(ctx, sql, id).Scan(
		&dailyMetric.ID,
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
	)
	if err != nil {
		return nil, my_errors.ErrDailyMetricNotFound
	}
	return &dailyMetric, nil
}

func (r *DailyMetricRepo) GetAll(ctx context.Context) ([]model.DailyMetric, error) {
	sql := `
	SELECT id, date, steps, ctl, atl, tsb, fatigure_score, readiness_score,
	body_baterry_avg, total_calories, sleep_score, stress_avg, reco,endation, update_at
	FROM daily_metrics
	`
	db := r.getDB(ctx)
	var dailyMetrics []model.DailyMetric
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return nil, my_errors.ErrDailyMetricNotFound
	}
	defer rows.Close()
	for rows.Next() {
		var dailyMetric model.DailyMetric
		err := rows.Scan(
			&dailyMetric.ID,
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
		)
		if err != nil {
			return nil, my_errors.ErrDailyMetricNotFound
		}
		dailyMetrics = append(dailyMetrics, dailyMetric)
	}
	return dailyMetrics, nil
}
