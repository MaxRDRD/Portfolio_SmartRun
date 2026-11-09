package repository

import (
	"SmartRun/internal/model"
	"context"
	"time"
)

type OutboxEventRepository interface {
	Add(ctx context.Context, outboxEvent *model.OutboxEvent) error
	FetchPending(ctx context.Context, batchSize int) ([]model.OutboxEvent, error)
	MarkAsPublished(ctx context.Context, id int64) error
	MarkAsFailed(ctx context.Context, id int64, errStr string, retryIn time.Duration) error
}
