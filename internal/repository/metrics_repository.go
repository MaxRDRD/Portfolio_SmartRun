package repository

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"context"
)

//go:generate mockery --name=MetricsRepository --dir=. --output=../usecase/service/mocks --outpkg=mocks --filename=mock_metrics_repository.go
type MetricsRepository interface {
	CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	DeleteMetrics(ctx context.Context, id int64, userID int64) error
	GetMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
	GetAllMetricsByID(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
}
