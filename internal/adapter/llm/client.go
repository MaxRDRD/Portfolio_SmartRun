package llm

import (
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"context"
	"fmt"

	"google.golang.org/genai"
)

// AICoach - интерфейс для генерации советов от ИИ-тренера
type AICoach interface {
	GenerateAdvice(ctx context.Context, metric *model.DailyMetric, recentWorkouts []model.Workouts) (string, error)
}

type geminiCoach struct {
	client *genai.Client
}

// NewGeminiCoach инициализирует клиент Gemini (требует переменную окружения GEMINI_API_KEY)
func NewGeminiCoach(ctx context.Context) (AICoach, error) {
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента gemini: %w", err)
	}
	return &geminiCoach{client: client}, nil
}

func (c *geminiCoach) GenerateAdvice(ctx context.Context, metric *model.DailyMetric, recentWorkouts []model.Workouts) (string, error) {
	log := logger.FromContext(ctx)

	// Формируем промпт
	prompt := fmt.Sprintf(`Ты - профессиональный тренер по бегу. Проанализируй данные спортсмена за сегодня и дай короткую рекомендацию (не более 3 предложений).

Сегодняшняя готовность:
- Readiness Score (Готовность, 0-100): %d
- Усталость (Fatigue, 0-100): %d
- Сон (оценка): %d, Часы сна: %.1f
- CTL (форма): %.1f
- ATL (острая усталость): %.1f
- TSB (баланс тренировочного стресса): %.1f

Недавние тренировки:
`, metric.ReadinessScore, metric.FatigueScore, metric.SleepScore, metric.SleepHours, metric.CTL, metric.ATL, metric.TSB)

	for i, w := range recentWorkouts {
		if i >= 3 {
			break // берем только 3 последние для контекста
		}
		prompt += fmt.Sprintf("- Дата: %s, Дистанция: %.1f км, Длительность: %d сек", w.Date.Format("2006-01-02"), w.Distance, w.Duration)
		if w.RPE != nil {
			prompt += fmt.Sprintf(", RPE: %d", *w.RPE)
		}
		if w.Feel != nil {
			prompt += fmt.Sprintf(", Самочувствие: %d", *w.Feel)
		}
		prompt += "\n"
	}

	prompt += "\nКакой совет ты дашь на сегодня? (Легкий бег, отдых, тяжелая тренировка?)"

	log.Info("Отправляем запрос к Gemini", "prompt_len", len(prompt))

	resp, err := c.client.Models.GenerateContent(ctx, "gemini-3.7-flash", genai.Text(prompt), nil)
	if err != nil {
		log.Error("ошибка при обращении к Gemini API", "error", err)
		return "", fmt.Errorf("GenerateAdvice: %w", err)
	}

	if resp.Text() == "" {
		return "", fmt.Errorf("пустой ответ от модели")
	}

	return resp.Text(), nil
}
