package metrics

import (
	"context"
	"errors"

	"github.com/go-playground/validator"
)

type Service interface {
	GetMetrics(ctx context.Context, filter MetricsFilter) ([]Metrics, error)
}

type service struct {
	repo     Repository
	validate *validator.Validate
}

func NewService(repo Repository) Service {
	validate := validator.New()

	return &service{
		repo:     repo,
		validate: validate,
	}
}

func (s *service) GetMetrics(ctx context.Context, filter MetricsFilter) ([]Metrics, error) {
	metrics, err := s.repo.GetMetrics(ctx, filter)
	if errors.Is(err, ErrMetricNotFound) {
		return nil, err
	}
	return metrics, err
}
