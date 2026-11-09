package model

import (
	"time"
)

const (
	StatusPending    OutboxStatus = "PENDING"
	StatusProcessing OutboxStatus = "PROCESSING"
	StatusPublished  OutboxStatus = "PUBLISHED"
	StatusFailed     OutboxStatus = "FAILED"
)

type OutboxStatus string

type OutboxEvent struct {
	ID           int64
	EventType    string
	ExchangeName string
	RoutingKey   string
	Payload      []byte
	Status       OutboxStatus
	RetryCount   int32
	MaxRetries   int32
	LastError    *string
	ScheduledAt  time.Time
	CreatedAt    time.Time
	ProcessedAt  *time.Time
}
