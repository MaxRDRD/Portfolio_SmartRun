package postgres

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"fmt"
)

type metricsRepository struct {
	db repository.DB
}

func NewMetricsRepository(db repository.DB) repository.MetricsRepository {
	return &metricsRepository{db: db}
}

func (r *metricsRepository) GetMetrics(ctx context.Context, filter dto.MetricsFilter) ([]model.Metrics, error) {
	sqlQuery := `
	SELECT
    COUNT(*) as total_workouts,
    SUM(distance) as total_distance,
    SUM(duration) as total_duration
	FROM workouts
	WHERE user_id = $1
	AND date BETWEEN $2 AND $3
	`

	rows, err := r.db.Query(ctx, sqlQuery, filter.UserID, filter.From, filter.To)
	if err != nil {
		return nil, fmt.Errorf("query all workouts: %w", err)
	}
	defer rows.Close()

	var metrics []model.Metrics

	for rows.Next() {
		var metric model.Metrics
		err := rows.Scan(
			&metric.ID,
			&metric.TotalDistance,
			&metric.TotalWorkouts,
			&metric.TotalDuration,
			&metric.AvgPace,
		)
		if err != nil {
			return nil, fmt.Errorf("scan workout: %w", err)
		}
		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return metrics, nil
}
