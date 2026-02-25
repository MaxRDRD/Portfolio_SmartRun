package workouts

import (
	"context"
	"errors"
	"time"

	"github.com/go-playground/validator"
)

type Service interface {
	Create(ctx context.Context, userID int, req CreateRequest) (*Workouts, error)
	GetByID(ctx context.Context, id int, userID int) (*Workouts, error)
	GetAllByID(ctx context.Context, filter WorkoutFilter) ([]Workouts, error)
	Delete(ctx context.Context, id, userID int) error
	Update(ctx context.Context, userID, id int, req UpdateRequest) (*Workouts, error)
	GetAll(ctx context.Context, filter WorkoutFilter) ([]Workouts, error)
}

type service struct {
	repo     Repository
	validate *validator.Validate
}

func NewService(repo Repository) Service {
	validate := validator.New()

	/*Регистрация нового формата
	_ = validate.RegisterValidation("date_format", func(fl validator.FieldLevel) bool {
		date := fl.Field().String()
		_, err := time.Parse("2006-01-02", date)
		return err == nil
	})
	*/
	return &service{
		repo:     repo,
		validate: validate,
	}
}

func (s *service) Create(ctx context.Context, userID int, req CreateRequest) (*Workouts, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	if req.Distance <= 0 {
		return nil, errors.New("distance must be greater than zero")
	}

	parsedDate, err := time.Parse("2006-01-02", req.Date)

	if err != nil {
		return nil, ErrInvalidData
	}

	workout := &Workouts{
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

func (s *service) GetByID(ctx context.Context, id int, userID int) (*Workouts, error) {
	workout, err := s.repo.GetByID(ctx, id, userID)
	if errors.Is(err, ErrWorkoutNotFound) {
		return nil, err
	}
	return workout, nil
}

func (s *service) GetAllByID(ctx context.Context, filter WorkoutFilter) ([]Workouts, error) {
	workouts, err := s.repo.GetAllByUserID(ctx, filter)
	if errors.Is(err, ErrWorkoutNotFound) {
		return nil, err
	}
	return workouts, nil
}

func (s *service) Delete(ctx context.Context, id, userID int) error {
	err := s.repo.DeleteWorkout(ctx, id, userID)
	return err
}

func (s *service) Update(ctx context.Context, userID, id int, req UpdateRequest) (*Workouts, error) {

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
			return nil, ErrInvalidData
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

func (s *service) GetAll(ctx context.Context, filter WorkoutFilter) ([]Workouts, error) {
	return s.repo.GetAllByUserID(ctx, filter)
}
