package service

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"time"

	"github.com/go-playground/validator"
)

type WorkoutService interface {
	Create(ctx context.Context, userID int, req dto.CreateRequest) (*model.Workouts, error)
	GetByID(ctx context.Context, id int, userID int) (*model.Workouts, error)
	GetAllByID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
	Delete(ctx context.Context, id, userID int) error
	Update(ctx context.Context, userID, id int, req dto.UpdateRequest) (*model.Workouts, error)
	GetAll(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
}

type workoutService struct {
	repo     repository.WorkoutRepository
	validate *validator.Validate
}

func NewWorkoutService(repo repository.WorkoutRepository) WorkoutService {
	validate := validator.New()

	/*Регистрация нового формата
	_ = validate.RegisterValidation("date_format", func(fl validator.FieldLevel) bool {
		date := fl.Field().String()
		_, err := time.Parse("2006-01-02", date)
		return err == nil
	})
	*/
	return &workoutService{
		repo:     repo,
		validate: validate,
	}
}

func (s *workoutService) Create(ctx context.Context, userID int, req dto.CreateRequest) (*model.Workouts, error) {
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
		UserID:       userID,
		Date:         parsedDate,
		Distance:     req.Distance,
		Duration:     req.Duration,
		TypeActivity: req.TypeActivity,
		Pace:         float64(req.Duration) / req.Distance,
	}

	err = s.repo.Create(ctx, workout)
	if err != nil {
		return nil, err
	}

	return workout, nil
}

func (s *workoutService) GetByID(ctx context.Context, id int, userID int) (*model.Workouts, error) {
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

func (s *workoutService) Delete(ctx context.Context, id, userID int) error {
	err := s.repo.DeleteWorkout(ctx, id, userID)
	return err
}

func (s *workoutService) Update(ctx context.Context, userID, id int, req dto.UpdateRequest) (*model.Workouts, error) {

	workout, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if req.Distance > 0 {
		workout.Distance = req.Distance
	}
	if req.Duration > 0 {
		workout.Duration = req.Duration
	}
	if req.TypeActivity != "" {
		workout.TypeActivity = req.TypeActivity
	}
	if req.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			return nil, my_errors.ErrInvalidData
		}
		workout.Date = parsedDate
	}

	workout.Pace = float64(workout.Duration) / workout.Distance

	err = s.repo.Update(ctx, workout)
	if err != nil {
		return nil, err
	}

	return workout, nil
}

func (s *workoutService) GetAll(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	return s.repo.GetAllByUserID(ctx, filter)
}
