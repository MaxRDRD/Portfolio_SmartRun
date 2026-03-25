package repository

import (
	"SmartRun/internal/model"
	"context"
	"time"
)

type DailyMetricRepository interface {
	Create(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*model.DailyMetric, error)
	GetAllByUserID(ctx context.Context, userId int64) ([]model.DailyMetric, error)
	GetByUserIDAndDate(ctx context.Context, userID int64, date time.Time) (*model.DailyMetric, error)
	UpdateOrCreate(ctx context.Context, dailyMetric *model.DailyMetric) error
}
