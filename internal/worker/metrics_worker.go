package worker

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/usecase/service"
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MetricsWorker struct {
	workoutService service.WorkoutService
}

func NewMetricsWorker(workoutService service.WorkoutService) *MetricsWorker {
	return &MetricsWorker{workoutService: workoutService}
}

func (w *MetricsWorker) ProcessMessage(msg amqp.Delivery) error {
	var event broker.MetricsRecalculationEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	log.Printf("MetricsWorker processing event for user %d, date %s", event.UserID, event.Date)

	ctx := context.Background()
	return w.workoutService.RecalculateDailyMetricsForDate(ctx, event.UserID, event.Date)
}
