package service

import (
	"SmartRun/internal/calculate"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
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
	workoutRepo      repository.WorkoutRepository
	dailyMetricsRepo repository.DailyMetricRepository
	userRepo         repository.UserRepository
	parser           importer.FitParser
	validate         *validator.Validate
	txManager        repository.TxManager
}

var minReasonableWorkoutDate = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func NewWorkoutService(workoutRepo repository.WorkoutRepository,
	dailyMetricsRepo repository.DailyMetricRepository,
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
		workoutRepo:      workoutRepo,
		dailyMetricsRepo: dailyMetricsRepo,
		userRepo:         userRepo,
		parser:           parser,
		validate:         validate,
		txManager:        txManager,
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
	if parsedDate.Before(minReasonableWorkoutDate) {
		return nil, errors.New("date must be on or after 1970-01-01")
	}

	var workout *model.Workouts

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {

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

		user, err := s.userRepo.GetUserByID(txCtx, userID)
		if err != nil {
			return my_errors.ErrUserNotFound
		}

		calculate.CalculateDerivedMetrics(workout, user)

		err = s.workoutRepo.Create(txCtx, workout)
		if err != nil {
			return err
		}

		// Пересчитать daily metrics для дня этой новой тренировки
		if err := s.recalculateDailyMetricsForDate(txCtx, userID, workout.Date); err != nil {
			return fmt.Errorf("recalculate daily metrics: %w", err)
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
	// Получить workout ДО удаления (нужна дата для пересчета metrics)
	workout, err := s.workoutRepo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}

	deleteDate := workout.Date

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Удалить workout
		if err := s.workoutRepo.DeleteWorkout(txCtx, id, userID); err != nil {
			return err
		}

		// Пересчитать daily metrics для дня, откуда удалили тренировку
		return s.recalculateDailyMetricsForDate(txCtx, userID, deleteDate)
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

	// Запомнили старую дату (может измениться)
	oldDate := workout.Date

	// 3. Применяем обновления в зависимости от метода
	if isFullReplace {
		// PUT: Полная замена — требуем ВСЕ required поля в запросе
		// Если основные поля пусты — это ошибка (не partial update)
		if req.Date == "" || req.Distance == 0 || req.Duration == 0 || req.TypeActivity == "" {
			return nil, errors.New("PUT requires all required fields: date, distance, duration, type_activity")
		}
	}
	// PATCH обновляет только пришедшие non-empty поля
	validupdate.ApplyUpdateRequest(workout, req)
	if workout.Date.Before(minReasonableWorkoutDate) {
		return nil, errors.New("date must be on or after 1970-01-01")
	}

	// 4. Пересчитываем ВСЕ производные метрики на основе актуального состояния
	calculate.CalculateDerivedMetrics(workout, user)

	var updatedWorkout *model.Workouts
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
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

		err := s.workoutRepo.Update(txCtx, workout)
		if err != nil {
			return err
		}
		updatedWorkout = workout

		// 6. Пересчитать daily metrics для дня изменённой тренировки
		if err := s.recalculateDailyMetricsForDate(txCtx, userID, workout.Date); err != nil {
			return fmt.Errorf("recalculate daily metrics: %w", err)
		}

		// 7. Если дата изменилась, пересчитать и старый день
		if oldDate != workout.Date {
			if err := s.recalculateDailyMetricsForDate(txCtx, userID, oldDate); err != nil {
				return fmt.Errorf("recalculate old date daily metrics: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return updatedWorkout, nil
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
	if activityData.StartTime.Before(minReasonableWorkoutDate) {
		return nil, errors.New("date must be on or after 1970-01-01")
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
	if err != nil {
		return nil, fmt.Errorf("upload FIT: %w", err)
	}
	if workout == nil || workout.ID <= 0 {
		return nil, fmt.Errorf("upload FIT: workout was not persisted")
	}

	// Пересчет daily metrics не должен откатывать уже сохраненную тренировку.
	if recalcErr := s.recalculateDailyMetricsForDate(ctx, userID, workout.Date); recalcErr != nil {
		logger.FromContext(ctx).Warn("workouts/upload-fit: daily metrics recalculation failed",
			"user_id", userID,
			"workout_id", workout.ID,
			"error", recalcErr,
		)
	}
	return workout, nil
}

// recalculateDailyMetricsForDate пересчитывает daily metrics для конкретного дня
// Получает все тренировки этого дня, пересчитывает метрики и сохраняет (INSERT или UPDATE)
func (s *workoutService) recalculateDailyMetricsForDate(ctx context.Context, userID int64, date time.Time) error {
	// Нормализуем дату (убираем время)
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := normalizedDate.AddDate(0, 0, 1).Add(-time.Nanosecond)

	// Получить все тренировки этого дня
	filter := dto.WorkoutFilter{
		UserID: userID,
		From:   &normalizedDate,
		To:     &dayEnd,
	}
	workouts, err := s.workoutRepo.GetAllByUserID(ctx, filter)
	if err != nil && !errors.Is(err, my_errors.ErrWorkoutNotFound) {
		return err
	}

	// Получить daily metric предыдущего дня (нужен для incremental streak и carry-over)
	yesterday := normalizedDate.AddDate(0, 0, -1)
	previousMetrics, err := s.dailyMetricsRepo.GetByUserIDAndDate(ctx, userID, yesterday)
	if err != nil {
		return err
	}
	// if previousMetrics == nil — это нормально, первый день пользователя

	// ===== ГЛАВНОЕ: используем calculate.CalculateDailyMetrics =====
	// Это функция автоматически:
	// - рассчитает CTL/ATL/TSB/Readiness на основе тренировок
	// - если нет тренировок → streak=0 но сохранит sleep/bodyBattery
	// - вычислит Monotony/Strain (вариативность и нагрузку)
	dailyMetric := calculate.CalculateDailyMetrics(workouts, previousMetrics)
	dailyMetric.UserID = userID
	dailyMetric.Date = normalizedDate

	// Merge с существующей записью если пользователь ввел метрики вручную
	existing, err := s.dailyMetricsRepo.GetByUserIDAndDate(ctx, userID, normalizedDate)
	if err != nil {
		return err
	}
	if existing != nil {
		// Сохраняем пользовательский ввод (не перезаписываем)
		if existing.SleepScore > 0 && dailyMetric.SleepScore == 0 {
			dailyMetric.SleepScore = existing.SleepScore
		}
		if existing.StressAvg > 0 && dailyMetric.StressAvg == 0 {
			dailyMetric.StressAvg = existing.StressAvg
		}
		if existing.BodyBatteryAvg > 0 && dailyMetric.BodyBatteryAvg == 0 {
			dailyMetric.BodyBatteryAvg = existing.BodyBatteryAvg
		}
		dailyMetric.ID = existing.ID
	}

	return s.dailyMetricsRepo.UpdateOrCreate(ctx, dailyMetric)
}
