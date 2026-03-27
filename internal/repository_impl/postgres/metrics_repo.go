package postgres

import (
	"SmartRun/internal/cache"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

type metricsRepository struct {
	db    repository.DB
	cache cache.Cache
	sf    singleflight.Group
}

func NewMetricsRepository(db repository.DB, cache cache.Cache) repository.MetricsRepository {
	return &metricsRepository{db: db, cache: cache}
}

func (r *metricsRepository) cachedStoredKey(userID int64) string {
	return fmt.Sprintf("metrics:stored:%d", userID)
}

func (r *metricsRepository) cacheGet(ctx context.Context, key string) (string, bool) {
	return cache.TryGetString(ctx, r.cache, key)
}

func (r *metricsRepository) cacheSetWithJitter(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	cache.SetJSONWithJitter(ctx, r.cache, key, value, ttl)
}

func (r *metricsRepository) invalidateStoredCache(ctx context.Context, userID int64) {
	cache.DelKey(ctx, r.cache, r.cachedStoredKey(userID))
}

func (r *metricsRepository) CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	log := logger.FromContext(ctx)
	sqlQuery := `
	INSERT INTO metrics (user_id, total_workouts, total_distance, total_duration, avg_pace, from_date, to_date, total_calories)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id
	`
	err := r.db.QueryRow(ctx, sqlQuery, metrics.UserID, metrics.TotalWorkouts, metrics.TotalDistance, metrics.TotalDuration, metrics.AvgPace, metrics.From, metrics.To, metrics.TotalCalories).Scan(&metrics.ID)
	if err != nil {
		log.Error("metrics repo: create failed", "user_id", metrics.UserID, "error", err)
		return nil, fmt.Errorf("create metrics: %w", err)
	}
	r.invalidateStoredCache(ctx, metrics.UserID)

	return &metrics, nil
}

func (r *metricsRepository) GetMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	log := logger.FromContext(ctx)
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
		log.Error("metrics repo: query aggregated failed", "user_id", filter.UserID, "error", err)
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
	log := logger.FromContext(ctx)
	sqlQuery := `
	UPDATE metrics
	SET total_workouts = $1, total_distance = $2, total_duration = $3,
	 avg_pace = $4,
	 from_date = COALESCE(NULLIF($5, '')::date, from_date),
	 to_date = COALESCE(NULLIF($6, '')::date, to_date),
	 total_calories = $7
	WHERE id = $8 AND user_id = $9
	`
	tag, err := r.db.Exec(ctx, sqlQuery,
		metrics.TotalWorkouts,
		metrics.TotalDistance,
		metrics.TotalDuration,
		metrics.AvgPace,
		metrics.From,
		metrics.To,
		metrics.TotalCalories,
		metrics.ID,
		metrics.UserID)
	if err != nil {
		log.Error("metrics repo: update failed", "id", metrics.ID, "user_id", metrics.UserID, "error", err)
		return nil, fmt.Errorf("update metrics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		log.Warn("metrics repo: update no rows affected", "id", metrics.ID, "user_id", metrics.UserID)
		return nil, fmt.Errorf("update metrics: no rows affected")
	}
	r.invalidateStoredCache(ctx, metrics.UserID)
	return &metrics, nil
}

func (r *metricsRepository) DeleteMetrics(ctx context.Context, id int64, userID int64) error {
	log := logger.FromContext(ctx)
	sqlQuery := `
	DELETE FROM metrics WHERE id = $1 AND user_id = $2
	`
	tag, err := r.db.Exec(ctx, sqlQuery, id, userID)
	if err != nil {
		log.Error("metrics repo: delete failed", "id", id, "user_id", userID, "error", err)
		return fmt.Errorf("delete metrics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		log.Warn("metrics repo: delete no rows affected", "id", id, "user_id", userID)
		return fmt.Errorf("delete metrics: no rows affected")
	}
	r.invalidateStoredCache(ctx, userID)
	return nil
}

func (r *metricsRepository) GetAllMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	log := logger.FromContext(ctx)
	cacheKey := r.cachedStoredKey(filter.UserID)
	if cached, ok := r.cacheGet(ctx, cacheKey); ok {
		var m model.Metrics
		if err := json.Unmarshal([]byte(cached), &m); err == nil {
			return &m, nil
		}
	}

	res, err, _ := r.sf.Do(cacheKey, func() (interface{}, error) {
		if cached, ok := r.cacheGet(ctx, cacheKey); ok {
			var m model.Metrics
			if unmarshalErr := json.Unmarshal([]byte(cached), &m); unmarshalErr == nil {
				return &m, nil
			}
		}
	sqlQuery := `
	SELECT id, user_id, total_workouts, total_distance, total_duration,
	avg_pace, from_date, to_date, total_calories
	FROM metrics
	WHERE user_id = $1
	ORDER BY to_date DESC, id DESC
	LIMIT 1
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
			log.Error("metrics repo: query stored failed", "user_id", filter.UserID, "error", err)
			return nil, fmt.Errorf("query metrics: %w", err)
		}
		metrics.From = fromDate.Format("2006-01-02")
		metrics.To = toDate.Format("2006-01-02")

		r.cacheSetWithJitter(ctx, cacheKey, &metrics, 5*time.Minute)
		return &metrics, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(*model.Metrics), nil

}
