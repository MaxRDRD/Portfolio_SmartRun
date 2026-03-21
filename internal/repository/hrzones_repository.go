package repository

import (
	"SmartRun/internal/model"
	"context"
)

type HRZonesRepository interface {
	Upsert(ctx context.Context, zones *model.WorkoutHRZones) error
	GetByWorkoutID(ctx context.Context, workoutID int64) (*model.WorkoutHRZones, error)
	DeleteByWorkoutID(ctx context.Context, workoutID int64) error
}
