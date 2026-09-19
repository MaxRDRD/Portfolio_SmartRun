package worker

import (
	"SmartRun/internal/adapter/llm"
	"SmartRun/internal/broker"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/usecase/service"
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AICoachWorker struct {
	coach              llm.AICoach
	dailyMetricService service.DailyMetricService
	workoutService     service.WorkoutService
}

func NewAICoachWorker(coach llm.AICoach, dmService service.DailyMetricService, wService service.WorkoutService) *AICoachWorker {
	return &AICoachWorker{
		coach:              coach,
		dailyMetricService: dmService,
		workoutService:     wService,
	}
}

func (w *AICoachWorker) ProcessMessage(msg amqp.Delivery) error {
	var event broker.MetricsRecalculationEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	ctx := context.Background()
	log := logger.FromContext(ctx)
	log.Info("AICoachWorker processing event", "user_id", event.UserID, "date", event.Date)

	// 1. Получаем DailyMetric за этот день
	metric, err := w.dailyMetricService.GetByUserIDAndDate(ctx, event.UserID, event.Date)
	if err != nil {
		log.Error("не удалось получить daily metric", "error", err)
		return fmt.Errorf("ProcessMessage: %w", err)
	}
	if metric == nil {
		log.Warn("метрика за день не найдена, пропускаем генерацию совета")
		return nil
	}

	// 2. Получаем недавние тренировки (за последние 14 дней, например)
	to := event.Date
	from := event.Date.AddDate(0, 0, -14)
	filter := dto.WorkoutFilter{
		UserID:    event.UserID,
		From:      &from,
		To:        &to,
		SortBy:    "date",
		SortOrder: "desc",
		Limit:     5, // берем до 5 последних
	}
	workouts, err := w.workoutService.GetAllByID(ctx, filter)
	if err != nil {
		log.Error("не удалось получить недавние тренировки", "error", err)
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	// 3. Обращаемся к Gemini для генерации совета
	advice, err := w.coach.GenerateAdvice(ctx, metric, workouts)
	if err != nil {
		log.Error("ошибка при генерации совета", "error", err)
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	// 4. Сохраняем совет в DailyMetric
	metric.Recommendation = advice

	updateReq := dto.UpdateDailyMetricRequest{
		ID:             int(metric.ID),
		UserID:         metric.UserID,
		Date:           metric.Date,
		CTL:            metric.CTL,
		ATL:            metric.ATL,
		TSB:            metric.TSB,
		FatigueScore:   metric.FatigueScore,
		ReadinessScore: metric.ReadinessScore,
		BodyBatteryAvg: metric.BodyBatteryAvg,
		Steps:          metric.Steps,
		TotalCalories:  metric.TotalCalories,
		SleepScore:     metric.SleepScore,
		SleepHours:     metric.SleepHours,
		StressAvg:      metric.StressAvg,
		Recommendation: metric.Recommendation,
		StreakDays:     metric.StreakDays,
		Monotony:       metric.Monotony,
		Strain:         metric.Strain,
	}

	_, err = w.dailyMetricService.UpdateDailyMetric(ctx, updateReq)
	if err != nil {
		log.Error("не удалось сохранить сгенерированный совет", "error", err)
		return fmt.Errorf("ProcessMessage: %w", err)
	}

	log.Info("Совет от ИИ успешно сгенерирован и сохранен", "user_id", event.UserID)
	return nil
}
