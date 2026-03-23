package service

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"

	"github.com/go-playground/validator"
)

type DailyMetricService interface {
	CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	//UpdateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	//DeleteDailyMetric(ctx context.Context, id int) error
	//GetDailyMetricByID(ctx context.Context, id int) (*model.DailyMetric, error)
}

type dailyMetricService struct {
	repo      repository.DailyMetricRepository
	validate  *validator.Validate
	txManager repository.TxManager
}

func NewDailyMetricService(repo repository.DailyMetricRepository, validator *validator.Validate, txManager repository.TxManager) DailyMetricService {

	return &dailyMetricService{
		repo:      repo,
		validate:  validator,
		txManager: txManager,
	}
}

func (s *dailyMetricService) CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	createdMetric, err := s.repo.Create(ctx, dailyMetric)
	if err != nil {
		return nil, err
	}
	return createdMetric, nil
}
