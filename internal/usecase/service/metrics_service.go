package service

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
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
	DeleteMetrics(ctx context.Context, id int64, userID int64) error
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
	log := logger.FromContext(ctx).With("user_id", metrics.UserID)
	createdMetrics, err := s.repo.CreateMetrics(ctx, metrics)
	if err != nil {
		log.Error("metrics service: create failed", "error", err)
		return nil, fmt.Errorf("create metrics: %w", err)
	}
	log.Info("metrics service: create success")
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
	log := logger.FromContext(ctx).With("user_id", filter.UserID)
	metric, err := s.repo.GetMetricsByID(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		log.Warn("metrics service: get not found")
		return nil, err
	}
	if err != nil {
		log.Error("metrics service: get failed", "error", err)
	}
	return metric, err
}

func (s *metricService) GetAllMetrics(ctx context.Context, filter dto.MetricsFilter) (*model.Metrics, error) {
	filter = normalizeMetricsFilter(filter)
	log := logger.FromContext(ctx).With("user_id", filter.UserID)
	metric, err := s.repo.GetAllMetricsByID(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		log.Warn("metrics service: get-all not found")
		return nil, err
	}
	if err != nil {
		log.Error("metrics service: get-all failed", "error", err)
	}
	return metric, err
}

func (s *metricService) UpdateMetrics(ctx context.Context, metrics model.Metrics) (*model.Metrics, error) {
	log := logger.FromContext(ctx).With("user_id", metrics.UserID, "metrics_id", metrics.ID)
	updatedMetrics, err := s.repo.UpdateMetrics(ctx, metrics)
	if err != nil {
		log.Error("metrics service: update failed", "error", err)
		return nil, fmt.Errorf("update metrics: %w", err)
	}
	log.Info("metrics service: update success")
	return updatedMetrics, nil
}

func (s *metricService) DeleteMetrics(ctx context.Context, id int64, userID int64) error {
	log := logger.FromContext(ctx).With("user_id", userID, "metrics_id", id)
	err := s.repo.DeleteMetrics(ctx, id, userID)
	if err != nil {
		log.Error("metrics service: delete failed", "error", err)
		return fmt.Errorf("delete metrics: %w", err)
	}
	log.Info("metrics service: delete success")
	return nil
}
