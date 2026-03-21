package importer

import (
	"SmartRun/internal/dto"
	"context"
	"time"
)

// FitParser — порт (абстракция), описывающий, что должен уметь любой парсер FIT-файлов
type FitParser interface {
	// Parse принимает сырые байты .fit-файла и возвращает структурированные данные
	Parse(ctx context.Context, data []byte) (*ActivityData, error)
}

// ActivityData — промежуточная структура, которая содержит только те данные,
// которые мы реально используем в нашей модели Workouts
type ActivityData struct {
	// Основные данные сессии
	StartTime    time.Time `json:"start_time"`    // дата и время начала тренировки
	Distance     float64   `json:"distance"`      // км
	Duration     int       `json:"duration"`      // секунды (TotalElapsedTime или TotalTimerTime)
	TypeActivity string    `json:"type_activity"` // "running", "cycling" и т.д. (Sport + SubSport)

	// Пульс
	AvgHR *int `json:"avg_hr"`
	MaxHR *int `json:"max_hr"`

	// Каденс
	AvgCadence *int `json:"avg_cadence"`
	MaxCadence *int `json:"max_cadence"`

	// Высота и подъём
	ElevationGain *float64 `json:"elevation_gain"` // метры
	ElevationLoss *float64 `json:"elevation_loss"` // метры (TotalDescent)

	// Калории
	Calories *int `json:"calories"` // TotalCalories

	// Дополнительные метрики (если есть в файле)
	TrainingLoad            *float64 `json:"training_load,omitempty"` // если Coros/Garmin записал
	RecoveryTime            *int     `json:"recovery_time,omitempty"` // минуты
	VO2MaxEstimate          *float64 `json:"vo2max_estimate,omitempty"`
	AerobicTrainingEffect   *float64 `json:"aerobic_training_effect,omitempty"`
	AnaerobicTrainingEffect *float64 `json:"anaerobic_training_effect,omitempty"`
	RPE                     *int     `json:"rpe,omitempty"` // если записано

	// Опционально: заметки / обувь (редко в FIT, но можно из developer fields)
	Notes *string `json:"notes,omitempty"`
	Shoes *string `json:"shoes,omitempty"`

	// Для будущего расширения (если захочешь хранить зоны или сплиты)
	HRZones *dto.HRZonesRequest `json:"hr_zones,omitempty"`
}
