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
	Update(ctx context.Context, userID, id int64, req dto.UpdateRequest, isFullReplace bool) (*model.Workouts, error)
	GetMonthlyHistory(ctx context.Context, userID int64, monthsLimit, monthsOffset int) ([]model.WorkoutMonthHistory, error)
	UploadFit(ctx context.Context, userID int64, fileData []byte) (*model.Workouts, error)
}

type workoutService struct {
	workoutRepo repository.WorkoutRepository
	userRepo    repository.UserRepository
	parser      importer.FitParser
	validate    *validator.Validate
	txManager   repository.TxManager
}

func NewWorkoutService(workoutRepo repository.WorkoutRepository,
	userRepo repository.UserRepository,
	parser importer.FitParser,
	validate *validator.Validate,
	txManager repository.TxManager) WorkoutService {

	/*Регистрация нового формата
	_ = validate.RegisterValidation("date_format", func(fl validator.FieldLevel) bool {
		date := fl.Field().String()
		_, err := time.Parse("2006-01-02", date)
		return err == nil
	})
	*/
	return &workoutService{
		workoutRepo: workoutRepo,
		userRepo:    userRepo,
		parser:      parser,
		validate:    validate,
		txManager:   txManager,
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

	var workout *model.Workouts

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {

		workout = &model.Workouts{
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
		if req.HRZones != nil {
			workout.TimeInHrZone = mapHRZonesRequest(req.HRZones)
		}

		user, err := s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			return my_errors.ErrUserNotFound
		}

		calculate.CalculateDerivedMetrics(workout, user)

		err = s.workoutRepo.Create(ctx, workout)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return workout, nil
}

func (s *workoutService) GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error) {
	workout, err := s.workoutRepo.GetByID(ctx, id, userID)
	if errors.Is(err, my_errors.ErrWorkoutNotFound) {
		return nil, err
	}
	return workout, nil
}

func (s *workoutService) GetAllByID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	workouts, err := s.workoutRepo.GetAllByUserID(ctx, filter)
	if errors.Is(err, my_errors.ErrWorkoutNotFound) {
		return nil, err
	}
	return workouts, nil
}

func (s *workoutService) Delete(ctx context.Context, id, userID int64) error {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.workoutRepo.DeleteWorkout(ctx, id, userID)
	})
	return err
}

func (s *workoutService) Update(ctx context.Context, userID, id int64, req dto.UpdateRequest, isFullReplace bool) (*model.Workouts, error) {

	// 1. Получаем пользователя (нужен для расчётов)
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, my_errors.ErrUserNotFound
	}

	// 2. Загружаем текущее состояние тренировки
	workout, err := s.workoutRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	// 3. Применяем только те поля, которые пришли (partial update)
	// === Вот и вся разница между PATCH и PUT ===
	if isFullReplace {
		// Полная замена: перезаписываем всё, что пришло
		validupdate.ApplyUpdateRequest(workout, req) // новый метод
	} else {
		// Частичное обновление (по умолчанию)
		validupdate.ApplyUpdateRequest(workout, req)
	}

	// 4. Пересчитываем ВСЕ производные метрики на основе актуального состояния
	calculate.CalculateDerivedMetrics(workout, user)

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		// 5. Если пришли зоны — сохраняем массивом прямо в workouts
		if req.HRZones != nil {
			workout.TimeInHrZone = mapHRZonesRequest(req.HRZones)

			// Сразу рассчитываем Training Effect на основе свежих зон
			aerobic, anaerobic := calculate.CalculateTrainingEffect(workout.TimeInHrZone, workout.Duration)
			workout.AerobicTrainingEffect = aerobic
			workout.AnaerobicTrainingEffect = anaerobic

			// Опционально: определяем основной фокус тренировки
			workout.PrimaryTrainingFocus = calculate.DeterminePrimaryFocus(*aerobic, *anaerobic, workout.TimeInHrZone)
		}

		err = s.workoutRepo.Update(ctx, workout)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return workout, nil
}

func mapHRZonesRequest(z *dto.HRZonesRequest) []int {
	if z == nil {
		return nil
	}

	zones := []int{z.Zone1Seconds, z.Zone2Seconds, z.Zone3Seconds, z.Zone4Seconds, z.Zone5Seconds}
	if z.Zone6Seconds > 0 {
		zones = append(zones, z.Zone6Seconds)
	}

	return zones
}

func (s *workoutService) GetMonthlyHistory(ctx context.Context, userID int64, monthsLimit, monthsOffset int) ([]model.WorkoutMonthHistory, error) {
	return s.workoutRepo.GetMonthlyHistory(ctx, userID, monthsLimit, monthsOffset)
}

func (s *workoutService) UploadFit(ctx context.Context, userID int64, fileData []byte) (*model.Workouts, error) {
	activityData, err := s.parser.Parse(ctx, fileData)
	if err != nil {
		return nil, fmt.Errorf("parse FIT: %w", err)
	}
	var workout *model.Workouts
	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		workout = &model.Workouts{
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
			TrainingStressScore:     activityData.TrainingStressScore,
			IntensityFactor:         activityData.IntensityFactor,
			AvgStress:               activityData.AvgStress,
			SdrrHrv:                 activityData.SdrrHrv,
			RmssdHrv:                activityData.RmssdHrv,
			TimeInHrZone:            activityData.TimeInHrZone,
			AerobicTrainingEffect:   activityData.AerobicTrainingEffect,
			AnaerobicTrainingEffect: activityData.AnaerobicTrainingEffect,
		}

		user, err := s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			return my_errors.ErrUserNotFound
		}

		calculate.CalculateDerivedMetrics(workout, user)

		err = s.workoutRepo.Create(ctx, workout)
		if err != nil {
			return err
		}
		return nil
	})
	return workout, nil
}
