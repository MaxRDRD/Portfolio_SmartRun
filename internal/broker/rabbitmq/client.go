package rabbitmq

import (
	"SmartRun/internal/broker"
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	MetricsQueue       = "metrics_recalculation"
	PasswordResetQueue = "password_reset"
	AICoachQueue       = "ai_coach_generate"
)

// RabbitMQPublisher структура для публикации сообщений в RabbitMQ
type RabbitMQPublisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewRabbitMQPublisher создает нового издателя RabbitMQ и объявляет необходимые очереди
func NewRabbitMQPublisher(url string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Объявление очередей
	_, err = ch.QueueDeclare(
		MetricsQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue %s: %w", MetricsQueue, err)
	}

	_, err = ch.QueueDeclare(
		PasswordResetQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue %s: %w", PasswordResetQueue, err)
	}

	_, err = ch.QueueDeclare(
		AICoachQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue %s: %w", AICoachQueue, err)
	}

	return &RabbitMQPublisher{
		conn: conn,
		ch:   ch,
	}, nil
}

// Close закрывает канал и соединение с RabbitMQ
func (p *RabbitMQPublisher) Close() error {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// publish отправляет сообщение в указанную очередь
func (p *RabbitMQPublisher) publish(ctx context.Context, queueName string, body []byte) error {
	return p.ch.PublishWithContext(
		ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// PublishMetricsRecalculation публикует событие перерасчета метрик
func (p *RabbitMQPublisher) PublishMetricsRecalculation(ctx context.Context, event broker.MetricsRecalculationEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("PublishMetricsRecalculation: %w", err)
	}
	log.Printf("Publishing metrics recalculation event for user %d, date %s", event.UserID, event.Date)
	return p.publish(ctx, MetricsQueue, b)
}

// PublishPasswordReset публикует событие сброса пароля
func (p *RabbitMQPublisher) PublishPasswordReset(ctx context.Context, event broker.PasswordResetEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("PublishPasswordReset: %w", err)
	}
	log.Printf("Publishing password reset event for %s", event.To)
	return p.publish(ctx, PasswordResetQueue, b)
}

// PublishAICoachEvent публикует событие для генерации советов ИИ-тренера
func (p *RabbitMQPublisher) PublishAICoachEvent(ctx context.Context, event broker.MetricsRecalculationEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("PublishAICoachEvent: %w", err)
	}
	log.Printf("Publishing AI coach event for user %d, date %s", event.UserID, event.Date)
	return p.publish(ctx, AICoachQueue, b)
}
