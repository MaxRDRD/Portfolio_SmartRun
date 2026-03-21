package service

import (
	"SmartRun/internal/calculate"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/ports/outgoing/importer"
	"SmartRun/internal/repository"
	validupdate "SmartRun/internal/validate"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator"
)

type WorkoutService interface {
	Create(ctx context.Context, userID int64, req dto.CreateRequest) (*model.Workouts, error)
	GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error)
	GetAllByID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
	Delete(ctx context.Context, id, userID int64) error
	Update(ctx context.Context, userID, id int64, req dto.UpdateRequest) (*model.Workouts, error)
	GetAll(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
	UploadFit(ctx context.Context, userID int64, fileData []byte) (*model.Workouts, error)
}

type workoutService struct {
	repo        repository.WorkoutRepository
	userRepo    repository.UserRepository
	hrZonesRepo repository.HRZonesRepository
	parser      importer.FitParser
	validate    *validator.Validate
}

func NewWorkoutService(repo repository.WorkoutRepository,
	userRepo repository.UserRepository,
	hrZonesRepo repository.HRZonesRepository,
	parser importer.FitParser,
	validate *validator.Validate) WorkoutService {

	/*Регистрация нового формата
	_ = validate.RegisterValidation("date_format", func(fl validator.FieldLevel) bool {
		date := fl.Field().String()
		_, err := time.Parse("2006-01-02", date)
		return err == nil
	})
	*/
	return &workoutService{
		repo:        repo,
		userRepo:    userRepo,
		hrZonesRepo: hrZonesRepo,
		parser:      parser,
		validate:    validate,
	}
}

func (s *workoutService) Create(ctx context.Context, userID int64, req dto.CreateRequest) (*model.Workouts, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	if req.Distance <= 0 {
		return nil, errors.New("distance must be greater than zero")
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)

	if err != nil {
		return nil, my_errors.ErrInvalidData
	}

	workout := &model.Workouts{
		UserID:        userID,
		Date:          parsedDate,
		Distance:      req.Distance,
		Duration:      req.Duration,
		TypeActivity:  req.TypeActivity,
		Calories:      req.Calories,
		AvgHR:         req.AvgHR,
		MaxHR:         req.MaxHR,
		AvgCadence:    req.AvgCadence,
		MaxCadence:    req.MaxCadence,
		ElevationGain: req.ElevationGain,
		ElevationLoss: req.ElevationLoss,
		RPE:           req.RPE,
	}
	if req.Notes != nil {
		workout.Notes = *req.Notes
	}
	if req.Shoes != nil {
		workout.Shoes = *req.Shoes
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, my_errors.ErrUserNotFound
	}

	calculate.CalculateDerivedMetrics(workout, user)

	err = s.repo.Create(ctx, workout)
	if err != nil {
		return nil, err
	}

	return workout, nil
}

func (s *workoutService) GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error) {
	workout, err := s.repo.GetByID(ctx, id, userID)
	if errors.Is(err, my_errors.ErrWorkoutNotFound) {
		return nil, err
	}
	return workout, nil
}

func (s *workoutService) GetAllByID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	workouts, err := s.repo.GetAllByUserID(ctx, filter)
	if errors.Is(err, my_errors.ErrWorkoutNotFound) {
		return nil, err
	}
	return workouts, nil
}

func (s *workoutService) Delete(ctx context.Context, id, userID int64) error {
	err := s.repo.DeleteWorkout(ctx, id, userID)
	return err
}

func (s *workoutService) Update(ctx context.Context, userID, id int64, req dto.UpdateRequest) (*model.Workouts, error) {

	// 1. Получаем пользователя (нужен для расчётов)
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, my_errors.ErrUserNotFound
	}

	// 2. Загружаем текущее состояние тренировки
	workout, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// 3. Применяем только те поля, которые пришли (partial update)
	validupdate.ApplyUpdateRequest(workout, req)

	// 4. Пересчитываем ВСЕ производные метрики на основе актуального состояния
	calculate.CalculateDerivedMetrics(workout, user)

	// 5. Если пришли зоны — обрабатываем их отдельно
	var zones *model.WorkoutHRZones
	if req.HRZones != nil {
		zones = &model.WorkoutHRZones{
			WorkoutID:    workout.ID,
			Zone1Seconds: req.HRZones.Zone1Seconds,
			Zone2Seconds: req.HRZones.Zone2Seconds,
			Zone3Seconds: req.HRZones.Zone3Seconds,
			Zone4Seconds: req.HRZones.Zone4Seconds,
			Zone5Seconds: req.HRZones.Zone5Seconds,
			// можно добавить CreatedAt / UpdatedAt, Source ("manual", "coros", "garmin") и т.д.
		}

		// Сохраняем / обновляем зоны (отдельный репозиторий)
		err = s.hrZonesRepo.Upsert(ctx, zones) // или Save / Update
		if err != nil {
			return nil, err
		}

		// Сразу рассчитываем Training Effect на основе свежих зон
		aerobic, anaerobic := calculate.CalculateTrainingEffect(zones, workout.Duration)
		workout.AerobicTrainingEffect = aerobic
		workout.AnaerobicTrainingEffect = anaerobic

		// Опционально: определяем основной фокус тренировки
		workout.PrimaryTrainingFocus = calculate.DeterminePrimaryFocus(*aerobic, *anaerobic, zones)
	}

	err = s.repo.Update(ctx, workout)
	if err != nil {
		return nil, err
	}

	return workout, nil
}

func (s *workoutService) GetAll(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	return s.repo.GetAllByUserID(ctx, filter)
}

func (s *workoutService) UploadFit(ctx context.Context, userID int64, fileData []byte) (*model.Workouts, error) {
	activityData, err := s.parser.Parse(ctx, fileData)
	if err != nil {
		return nil, fmt.Errorf("parse FIT: %w", err)
	}

	workout := &model.Workouts{
		UserID:                  userID,
		Date:                    activityData.StartTime,
		Distance:                activityData.Distance,
		Duration:                activityData.Duration,
		TypeActivity:            activityData.TypeActivity,
		Calories:                activityData.Calories,
		AvgHR:                   activityData.AvgHR,
		MaxHR:                   activityData.MaxHR,
		AvgCadence:              activityData.AvgCadence,
		ElevationGain:           activityData.ElevationGain,
		ElevationLoss:           activityData.ElevationLoss,
		AerobicTrainingEffect:   activityData.AerobicTrainingEffect,
		AnaerobicTrainingEffect: activityData.AnaerobicTrainingEffect,
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, my_errors.ErrUserNotFound
	}

	calculate.CalculateDerivedMetrics(workout, user)

	err = s.repo.Create(ctx, workout)
	if err != nil {
		return nil, err
	}

	return workout, nil
}
