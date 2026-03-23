package repository

import (
	"SmartRun/internal/model"
	"context"
)

type DailyMetricRepository interface {
	Create(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	Update(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*model.DailyMetric, error)
	GetAll(ctx context.Context) ([]model.DailyMetric, error)
}
