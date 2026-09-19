package service

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/calculate"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator"
)

type DailyMetricService interface {
	CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error)
	UpdateDailyMetric(ctx context.Context, req dto.UpdateDailyMetricRequest) (*model.DailyMetric, error)
	DeleteDailyMetric(ctx context.Context, id int) error
	GetDailyMetricByID(ctx context.Context, id int) (*model.DailyMetric, error)
	GetByUserIDAndDate(ctx context.Context, userID int64, date time.Time) (*model.DailyMetric, error)
	GetAllDailyMetrics(ctx context.Context, userId int64) ([]model.DailyMetric, error)
	TriggerAdviceGeneration(ctx context.Context, userID int64, date time.Time) error
}

type dailyMetricService struct {
	dailyMetricRepo repository.DailyMetricRepository
	workoutRepo     repository.WorkoutRepository
	validate        *validator.Validate
	txManager       repository.TxManager
	publisher       broker.Publisher
}

func NewDailyMetricService(repo repository.DailyMetricRepository,
	workoutRepo repository.WorkoutRepository,
	validator *validator.Validate,
	txManager repository.TxManager,
	publisher broker.Publisher) DailyMetricService {

	return &dailyMetricService{
		dailyMetricRepo: repo,
		workoutRepo:     workoutRepo,
		validate:        validator,
		txManager:       txManager,
		publisher:       publisher,
	}
}

func (s *dailyMetricService) CreateDailyMetric(ctx context.Context, dailyMetric model.DailyMetric) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	createdMetric, err := s.dailyMetricRepo.Create(ctx, dailyMetric)
	if err != nil {
		log.Error("daily metrics service: create failed", "user_id", dailyMetric.UserID, "date", dailyMetric.Date.Format("2006-01-02"), "error", err)
		return nil, fmt.Errorf("CreateDailyMetric: %w", err)
	}
	log.Info("daily metrics service: create success", "user_id", dailyMetric.UserID, "id", createdMetric.ID)
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
	log := logger.FromContext(ctx)
	existing, err := s.dailyMetricRepo.GetByID(ctx, req.ID)
	if err != nil {
		log.Error("daily metrics service: get existing failed", "id", req.ID, "error", err)
		return nil, fmt.Errorf("UpdateDailyMetric: %w", err)
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
		log.Error("daily metrics service: workout fetch failed", "user_id", userID, "error", err)
		return nil, fmt.Errorf("UpdateDailyMetric: %w", err)
	}

	if req.SleepScore > 0 {
		existing.SleepScore = req.SleepScore
	}
	if req.SleepHours > 0 {
		existing.SleepHours = req.SleepHours
	}
	if req.StressAvg > 0 {
		existing.StressAvg = req.StressAvg
	}
	if req.BodyBatteryAvg > 0 {
		existing.BodyBatteryAvg = req.BodyBatteryAvg
	}
	if req.Steps > 0 {
		existing.Steps = req.Steps
	}

	updated := calculate.CalculateDailyMetrics(workouts, existing)
	updated.ID = existing.ID
	updated.UserID = userID
	updated.Date = anchorDate

	result, err := s.dailyMetricRepo.Update(ctx, *updated)
	if err != nil {
		log.Error("daily metrics service: update failed", "id", req.ID, "error", err)
		return nil, fmt.Errorf("UpdateDailyMetric: %w", err)
	}
	log.Info("daily metrics service: update success", "id", req.ID, "user_id", userID)
	return result, nil
}

func (s *dailyMetricService) DeleteDailyMetric(ctx context.Context, id int) error {
	log := logger.FromContext(ctx)
	err := s.dailyMetricRepo.Delete(ctx, id)
	if err != nil {
		log.Error("daily metrics service: delete failed", "id", id, "error", err)
		return fmt.Errorf("DeleteDailyMetric: %w", err)
	}
	log.Info("daily metrics service: delete success", "id", id)
	return nil
}

func (s *dailyMetricService) GetDailyMetricByID(ctx context.Context, id int) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	dailyMetric, err := s.dailyMetricRepo.GetByID(ctx, id)
	if err != nil {
		log.Error("daily metrics service: get-by-id failed", "id", id, "error", err)
		return nil, fmt.Errorf("GetDailyMetricByID: %w", err)
	}
	return dailyMetric, nil
}

func (s *dailyMetricService) GetByUserIDAndDate(ctx context.Context, userID int64, date time.Time) (*model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	dailyMetric, err := s.dailyMetricRepo.GetByUserIDAndDate(ctx, userID, date)
	if err != nil {
		log.Error("daily metrics service: get-by-user-date failed", "user_id", userID, "date", date, "error", err)
		return nil, fmt.Errorf("GetByUserIDAndDate: %w", err)
	}
	return dailyMetric, nil
}

func (s *dailyMetricService) GetAllDailyMetrics(ctx context.Context, userId int64) ([]model.DailyMetric, error) {
	log := logger.FromContext(ctx)
	dailyMetrics, err := s.dailyMetricRepo.GetAllByUserID(ctx, userId)
	if err != nil {
		log.Error("daily metrics service: get-all failed", "user_id", userId, "error", err)
		return nil, fmt.Errorf("GetAllDailyMetrics: %w", err)
	}
	log.Info("daily metrics service: get-all success", "user_id", userId, "count", len(dailyMetrics))
	return dailyMetrics, nil
}

func (s *dailyMetricService) TriggerAdviceGeneration(ctx context.Context, userID int64, date time.Time) error {
	log := logger.FromContext(ctx)
	event := broker.MetricsRecalculationEvent{
		UserID: userID,
		Date:   date,
	}
	if err := s.publisher.PublishAICoachEvent(ctx, event); err != nil {
		log.Error("не удалось опубликовать событие для ИИ-тренера", "user_id", userID, "error", err)
		return fmt.Errorf("TriggerAdviceGeneration: %w", err)
	}
	log.Info("успешно опубликовано событие для генерации совета ИИ-тренера", "user_id", userID)
	return nil
}
