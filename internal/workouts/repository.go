package workouts

import "context"

type Repository interface {
	Create(ctx context.Context, user *Workouts) error
	GetByID(ctx context.Context, id int) (*Workouts, error)
}
