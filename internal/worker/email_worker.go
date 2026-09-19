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

type EmailWorker struct {
	emailService service.EmailService
}

func NewEmailWorker(emailService service.EmailService) *EmailWorker {
	return &EmailWorker{emailService: emailService}
}

func (w *EmailWorker) ProcessMessage(msg amqp.Delivery) error {
	var event broker.PasswordResetEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	log.Printf("EmailWorker processing password reset email for %s", event.To)

	ctx := context.Background()
	return w.emailService.SendPasswordResetEmail(ctx, event.To, event.ResetLink, event.UserName)
}
