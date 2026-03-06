package service

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"

	"github.com/go-playground/validator"
)

type MetricService interface {
	GetMetrics(ctx context.Context, filter dto.MetricsFilter) ([]model.Metrics, error)
}

type metricService struct {
	repo     repository.MetricsRepository
	validate *validator.Validate
}

func NewMetricsService(repo repository.MetricsRepository) MetricService {
	validate := validator.New()

	return &metricService{
		repo:     repo,
		validate: validate,
	}
}

func (s *metricService) GetMetrics(ctx context.Context, filter dto.MetricsFilter) ([]model.Metrics, error) {
	metrics, err := s.repo.GetMetrics(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		return nil, err
	}
	return metrics, err
}
