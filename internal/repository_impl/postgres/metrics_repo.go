package postgres

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"fmt"
	"time"
)

type metricsRepository struct {
	db repository.DB
}

func NewMetricsRepository(db repository.DB) repository.MetricsRepository {
	return &metricsRepository{db: db}
}

func (r *metricsRepository) CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	sqlQuery := `
	INSERT INTO metrics (user_id, total_workouts, total_distance, total_duration, avg_pace, from_date, to_date, total_calories)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, sqlQuery, metrics.UserID, metrics.TotalWorkouts, metrics.TotalDistance, metrics.TotalDuration, metrics.AvgPace, metrics.From, metrics.To, metrics.TotalCalories)
	if err != nil {
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	return &metrics, nil
}

func (r *metricsRepository) GetMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	sqlQuery := `
	SELECT
    COALESCE(COUNT(*), 0) as total_workouts,
    COALESCE(SUM(distance), 0) as total_distance,
    COALESCE(SUM(duration), 0) as total_duration,
    COALESCE(SUM(calories), 0) as total_calories
	FROM workouts
	WHERE user_id = $1
	AND date BETWEEN $2 AND $3
	`

	var totalWorkouts int
	var totalDistance float64
	var totalDuration int
	var totalCalories int64

	err := r.db.QueryRow(ctx, sqlQuery, filter.UserID, filter.From, filter.To).Scan(
		&totalWorkouts,
		&totalDistance,
		&totalDuration,
		&totalCalories,
	)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}

	var avgPace float64
	if totalDuration > 0 && totalDistance > 0 {
		// Avg pace in min/km.
		avgPace = (float64(totalDuration) / 60.0) / totalDistance
	}

	// Вернём нормализованный результат - одну метрику (агрегированные данные по юзеру и периоду)
	var fromStr, toStr string
	if filter.From != nil {
		fromStr = filter.From.Format("2006-01-02")
	}
	if filter.To != nil {
		toStr = filter.To.Format("2006-01-02")
	}

	metric := &model.Metrics{
		ID:            filter.UserID, // используем UserID как ID
		UserID:        filter.UserID,
		TotalWorkouts: totalWorkouts,
		TotalDistance: totalDistance,
		TotalDuration: totalDuration,
		AvgPace:       avgPace,
		From:          fromStr,
		To:            toStr,
		TotalCalories: totalCalories,
	}

	return metric, nil
}

func (r *metricsRepository) UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	sqlQuery := `
	UPDATE metrics
	SET total_workouts = $1, total_distance = $2, total_duration = $3,
	 avg_pace = $4,
	 from_date = COALESCE(NULLIF($5, '')::date, from_date),
	 to_date = COALESCE(NULLIF($6, '')::date, to_date),
	 total_calories = $7
	WHERE user_id = $8
	`
	tag, err := r.db.Exec(ctx, sqlQuery,
		metrics.TotalWorkouts,
		metrics.TotalDistance,
		metrics.TotalDuration,
		metrics.AvgPace,
		metrics.From,
		metrics.To,
		metrics.TotalCalories,
		metrics.UserID)
	if err != nil {
		return nil, fmt.Errorf("update metrics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("update metrics: no rows affected")
	}
	return &metrics, nil
}

func (r *metricsRepository) DeleteMetrics(ctx context.Context, id int) error {
	sqlQuery := `
	DELETE FROM metrics WHERE user_id = $1	
	`
	tag, err := r.db.Exec(ctx, sqlQuery, id)
	if err != nil {
		return fmt.Errorf("delete metrics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete metrics: no rows affected")
	}
	return nil
}

func (r *metricsRepository) GetAllMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	sqlQuery := `
	SELECT id, user_id, total_workouts, total_distance, total_duration,
	avg_pace, from_date, to_date, total_calories
	FROM metrics
	WHERE user_id = $1
	`
	var metrics model.Metrics
	var fromDate time.Time
	var toDate time.Time
	err := r.db.QueryRow(ctx, sqlQuery, filter.UserID).Scan(
		&metrics.ID,
		&metrics.UserID,
		&metrics.TotalWorkouts,
		&metrics.TotalDistance,
		&metrics.TotalDuration,
		&metrics.AvgPace,
		&fromDate,
		&toDate,
		&metrics.TotalCalories,
	)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	metrics.From = fromDate.Format("2006-01-02")
	metrics.To = toDate.Format("2006-01-02")
	return &metrics, nil

}
