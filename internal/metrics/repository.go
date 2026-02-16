package metrics

import "context"

type Repository interface {
	Create(ctx context.Context, user *Metrics) error
	GetByID(ctx context.Context, id int) (*Metrics, error)
}
