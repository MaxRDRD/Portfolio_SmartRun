package broker

import (
	"context"
	"time"
)

// MetricsRecalculationEvent структура события для перерасчета ежедневных метрик
type MetricsRecalculationEvent struct {
	UserID int64     `json:"user_id"`
	Date   time.Time `json:"date"`
}

// PasswordResetEvent структура события для отправки письма сброса пароля
type PasswordResetEvent struct {
	To        string `json:"to"`
	ResetLink string `json:"reset_link"`
	UserName  string `json:"user_name"`
}

// Publisher интерфейс для публикации доменных событий в брокер сообщений
type Publisher interface {
	// PublishMetricsRecalculation публикует событие перерасчета метрик
	PublishMetricsRecalculation(ctx context.Context, event MetricsRecalculationEvent) error
	// PublishPasswordReset публикует событие сброса пароля
	PublishPasswordReset(ctx context.Context, event PasswordResetEvent) error
	// PublishAICoachEvent публикует событие для генерации советов ИИ-тренера
	PublishAICoachEvent(ctx context.Context, event MetricsRecalculationEvent) error
}
