package repository

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"context"
)

type MetricsRepository interface {
	GetMetrics(ctx context.Context, filter dto.MetricsFilter) ([]model.Metrics, error)
}
