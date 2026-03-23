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

func (r *metricsRepository) GetMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
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
