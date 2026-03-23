package repository

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"context"
)

type WorkoutRepository interface {
	Create(ctx context.Context, workout *model.Workouts) error
	GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error)
	GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
	DeleteWorkout(ctx context.Context, id int64, userID int64) error
	Update(ctx context.Context, workout *model.Workouts) error
	GetMonthlyHistory(ctx context.Context, userID int64, monthsLimit, monthsOffset int) ([]model.WorkoutMonthHistory, error)
}
