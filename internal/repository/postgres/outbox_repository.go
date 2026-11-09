package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"context"
	"fmt"
	"time"
)

type outboxEventRepository struct {
	db repository.DB
}

func NewOutboxEventRepository(db repository.DB) repository.OutboxEventRepository {
	return &outboxEventRepository{db: db}
}

func (r *outboxEventRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db
}

func (r *outboxEventRepository) Add(ctx context.Context, event *model.OutboxEvent) error {
	db := r.getDB(ctx)

	now := time.Now().UTC()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.ScheduledAt.IsZero() {
		event.ScheduledAt = now
	}
	if event.Status == "" {
		event.Status = model.StatusPending
	}
	if event.MaxRetries <= 0 {
		event.MaxRetries = 5
	}

	query := `
		INSERT INTO outbox_events (
			event_type,
			exchange_name,
			routing_key,
			payload,
			status,
			retry_count,
			max_retries,
			scheduled_at,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING id, created_at;
	`

	err := db.QueryRow(ctx, query,
		event.EventType,
		event.ExchangeName,
		event.RoutingKey,
		event.Payload,
		event.Status,
		event.RetryCount,
		event.MaxRetries,
		event.ScheduledAt,
		event.CreatedAt,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return fmt.Errorf("outboxRepo.Add: %w", err)
	}

	return nil
}

func (r *outboxEventRepository) FetchPending(ctx context.Context, batchSize int) ([]model.OutboxEvent, error) {
	db := r.getDB(ctx)

	query := `
		SELECT 
			id, event_type, exchange_name, routing_key, payload, 
			status, retry_count, max_retries, last_error, scheduled_at, created_at, processed_at
		FROM outbox_events
		WHERE status IN ('PENDING', 'FAILED') 
		  AND retry_count < max_retries 
		  AND scheduled_at <= NOW()
		ORDER BY id ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`

	rows, err := db.Query(ctx, query, batchSize)
	if err != nil {
		return nil, fmt.Errorf("outboxRepo.FetchPending: %w", err)
	}
	defer rows.Close()

	var events []model.OutboxEvent
	for rows.Next() {
		var event model.OutboxEvent
		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.ExchangeName,
			&event.RoutingKey,
			&event.Payload,
			&event.Status,
			&event.RetryCount,
			&event.MaxRetries,
			&event.LastError,
			&event.ScheduledAt,
			&event.CreatedAt,
			&event.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("outboxRepo.FetchPending: scan error: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("outboxRepo.FetchPending: rows iteration: %w", err)
	}

	return events, nil
}

func (r *outboxEventRepository) MarkAsPublished(ctx context.Context, id int64) error {
	db := r.getDB(ctx)

	query := `
		UPDATE outbox_events
		SET status = 'PUBLISHED',
		    processed_at = NOW(),
		    last_error = NULL
		WHERE id = $1;
	`

	_, err := db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("outboxRepo.MarkAsPublished: %w", err)
	}

	return nil
}

func (r *outboxEventRepository) MarkAsFailed(ctx context.Context, id int64, errStr string, retryIn time.Duration) error {
	db := r.getDB(ctx)

	retrySeconds := int64(retryIn.Seconds())
	if retrySeconds <= 0 {
		retrySeconds = 1
	}

	query := `
		UPDATE outbox_events
		SET retry_count = retry_count + 1,
		    last_error = $2,
		    scheduled_at = NOW() + ($3 * INTERVAL '1 second'),
		    status = CASE 
		        WHEN retry_count + 1 >= max_retries THEN 'FAILED'::varchar
		        ELSE 'PENDING'::varchar
		    END
		WHERE id = $1;
	`

	_, err := db.Exec(ctx, query, id, errStr, retrySeconds)
	if err != nil {
		return fmt.Errorf("outboxRepo.MarkAsFailed: %w", err)
	}

	return nil
}
