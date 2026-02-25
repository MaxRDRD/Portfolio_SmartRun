package metrics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetMetrics(ctx context.Context, filter MetricsFilter) ([]Metrics, error)
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) GetMetrics(ctx context.Context, filter MetricsFilter) ([]Metrics, error) {
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

	var metrics []Metrics

	for rows.Next() {
		var metric Metrics
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
