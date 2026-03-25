package repository

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"context"
)

type MetricsRepository interface {
	CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	DeleteMetrics(ctx context.Context, id int) error
	GetMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
	GetAllMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
}
