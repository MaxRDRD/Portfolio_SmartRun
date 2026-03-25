package service

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator"
)

type MetricService interface {
	CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	GetMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
	GetAllMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error)
	UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error)
	DeleteMetrics(ctx context.Context, id int) error
}

type metricService struct {
	repo     repository.MetricsRepository
	validate *validator.Validate
}

func NewMetricsService(repo repository.MetricsRepository, validator *validator.Validate) MetricService {

	return &metricService{
		repo:     repo,
		validate: validator,
	}
}

func (s *metricService) CreateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	createdMetrics, err := s.repo.CreateMetrics(ctx, metrics)
	if err != nil {
		return nil, fmt.Errorf("create metrics: %w", err)
	}
	return createdMetrics, nil
}

func normalizeMetricsFilter(filter dto.MetricsFilter) dto.MetricsFilter {
	now := time.Now().UTC()

	if filter.To == nil {
		to := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
		filter.To = &to
	} else {
		to := filter.To.UTC()
		to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
		filter.To = &to
	}

	if filter.From == nil {
		from := filter.To.AddDate(0, 0, -30)
		from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		filter.From = &from
	} else {
		from := filter.From.UTC()
		from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		filter.From = &from
	}

	if filter.From.After(*filter.To) {
		filter.From = filter.To
	}

	return filter
}

func (s *metricService) GetMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	filter = normalizeMetricsFilter(filter)
	metric, err := s.repo.GetMetricsByID(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		return nil, err
	}
	return metric, err
}

func (s *metricService) GetAllMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	filter = normalizeMetricsFilter(filter)
	metric, err := s.repo.GetAllMetricsByID(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		return nil, err
	}
	return metric, err
}

func (s *metricService) UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	updatedMetrics, err := s.repo.UpdateMetrics(ctx, metrics)
	if err != nil {
		return nil, fmt.Errorf("update metrics: %w", err)
	}
	return updatedMetrics, nil
}

func (s *metricService) DeleteMetrics(ctx context.Context, id int) error {
	err := s.repo.DeleteMetrics(ctx, id)
	if err != nil {
		return fmt.Errorf("delete metrics: %w", err)
	}
	return nil
}
