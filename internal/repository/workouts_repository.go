package repository

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"context"
)

type WorkoutRepository interface {
	Create(ctx context.Context, workour *model.Workouts) error
	GetByID(ctx context.Context, id int, userID int) (*model.Workouts, error)
	GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error)
	DeleteWorkout(ctx context.Context, id int, userID int) error
	Update(ctx context.Context, workout *model.Workouts) error
}
