package service

import (
	"SmartRun/internal/calculate"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"time"

	"github.com/go-playground/validator"
)

type DailyMetricService interface {
	CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	UpdateDailyMetric(ctx context.Context, req dto.UpdateDailyMetricRequest) (*model.DailyMetric, error)
	DeleteDailyMetric(ctx context.Context, id int) error
	GetDailyMetricByID(ctx context.Context, id int) (*model.DailyMetric, error)
	GetAllDailyMetrics(ctx context.Context, userId int64) ([]model.DailyMetric, error)
}

type dailyMetricService struct {
	dailyMetricRepo repository.DailyMetricRepository
	workoutRepo     repository.WorkoutRepository
	validate        *validator.Validate
	txManager       repository.TxManager
}

func NewDailyMetricService(repo repository.DailyMetricRepository,
	workoutRepo repository.WorkoutRepository,
	validator *validator.Validate,
	txManager repository.TxManager) DailyMetricService {

	return &dailyMetricService{
		dailyMetricRepo: repo,
		workoutRepo:     workoutRepo,
		validate:        validator,
		txManager:       txManager,
	}
}

func (s *dailyMetricService) CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	createdMetric, err := s.dailyMetricRepo.Create(ctx, dailyMetric)
	if err != nil {
		return nil, err
	}
	return createdMetric, nil
}

func normalizeDateUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func endOfDayUTC(t time.Time) time.Time {
	t = normalizeDateUTC(t)
	return t.AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func (s *dailyMetricService) UpdateDailyMetric(ctx context.Context, req dto.UpdateDailyMetricRequest) (*model.DailyMetric, error) {
	existing, err := s.dailyMetricRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	anchorDate := normalizeDateUTC(req.Date)
	if req.Date.IsZero() {
		anchorDate = normalizeDateUTC(existing.Date)
	}

	userID := req.UserID
	if userID == 0 {
		userID = existing.UserID
	}

	to := endOfDayUTC(anchorDate)
	from := normalizeDateUTC(anchorDate.AddDate(0, 0, -42))
	filter := dto.WorkoutFilter{
		UserID:    userID,
		From:      &from,
		To:        &to,
		SortBy:    "date",
		SortOrder: "desc",
	}

	workouts, err := s.workoutRepo.GetAllByUserID(ctx, filter)
	if err != nil {
		return nil, err
	}

	updated := calculate.CalculateDailyMetrics(workouts, existing)
	updated.ID = existing.ID
	updated.UserID = userID
	updated.Date = anchorDate

	return s.dailyMetricRepo.Update(ctx, *updated)
}

func (s *dailyMetricService) DeleteDailyMetric(ctx context.Context, id int) error {
	return s.dailyMetricRepo.Delete(ctx, id)
}

func (s *dailyMetricService) GetDailyMetricByID(ctx context.Context, id int) (*model.DailyMetric, error) {
	dailyMetric, err := s.dailyMetricRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dailyMetric, nil
}

func (s *dailyMetricService) GetAllDailyMetrics(ctx context.Context, userId int64) ([]model.DailyMetric, error) {
	dailyMetrics, err := s.dailyMetricRepo.GetAllByUserID(ctx, userId)
	if err != nil {
		return nil, err
	}
	return dailyMetrics, nil
}
