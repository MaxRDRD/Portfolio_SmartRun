package calculate_test

import (
	"SmartRun/internal/calculate"
	"SmartRun/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateDerivedMetrics_Pace(t *testing.T) {
	tests := []struct {
		name     string
		workout  *model.Workouts
		wantPace float64
	}{
		{
			name: "good_test: корректный темп 5 мин/км за 3000 сек на 10 км",
			workout: &model.Workouts{
				Distance: 10.0,
				Duration: 3000,
			},
			wantPace: 5.0,
		},
		{
			name: "good_test: корректный темп 5.5 мин/км за 55 минут на 10 км",
			workout: &model.Workouts{
				Distance: 10.0,
				Duration: 3300,
			},
			wantPace: 5.5,
		},
		{
			name: "good_test: нулевая дистанция",
			workout: &model.Workouts{
				Distance: 0,
				Duration: 3000,
			},
			wantPace: 0.0,
		},
		{
			name: "good_test: нулевое время",
			workout: &model.Workouts{
				Distance: 1000,
				Duration: 0,
			},
			wantPace: 0.0,
		},
		{
			name: "good_test: отрицательное время",
			workout: &model.Workouts{
				Distance: 1000,
				Duration: -100,
			},
			wantPace: 0.0,
		},
		{
			name: "good_test: отрицательная дистанция",
			workout: &model.Workouts{
				Distance: -1000,
				Duration: 100,
			},
			wantPace: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, &model.User{})

			assert.InDelta(t, tt.wantPace, tt.workout.Pace, 1e-4)
		})
	}
}

func TestCalculateDerivedMetrics_Calories(t *testing.T) {
	tests := []struct {
		name         string
		workout      *model.Workouts
		user         *model.User
		wantCalories *int
	}{
		{
			name: "Расчет каллорий без уклона",
			workout: &model.Workouts{
				Distance:      10.0,
				Duration:      3000,
				ElevationGain: nil,
			},
			user: &model.User{
				WeightKg: 70,
			},
			wantCalories: Ptr(761),
		},
		{
			name: "Расчет каллорий с уклоном",
			workout: &model.Workouts{
				Distance:      5.0,
				Duration:      1500,
				ElevationGain: Ptr(100.0),
			},
			user: &model.User{
				WeightKg: 75,
			},
			wantCalories: Ptr(442),
		},
		{
			name: "каллории уже указаны в тренировке",
			workout: &model.Workouts{
				Distance:      5.0,
				Duration:      1500,
				ElevationGain: Ptr(100.0),
				Calories:      Ptr(600),
			},
			user: &model.User{
				WeightKg: 75,
			},
			wantCalories: Ptr(600),
		},
		{
			name: "невозможен расчет каллорий тк не указан вкс пользователя",
			workout: &model.Workouts{
				Distance:      5.0,
				Duration:      1500,
				ElevationGain: Ptr(100.0),
			},
			user: &model.User{
				WeightKg: 0,
			},
			wantCalories: nil,
		},
		{
			name: "невозможен расчет каллорий тк нулевая дистанция",
			workout: &model.Workouts{
				Distance:      0,
				Duration:      1500,
				ElevationGain: Ptr(100.0),
			},
			user: &model.User{
				WeightKg: 75,
			},
			wantCalories: nil,
		},
		{
			name: "невозможен расчет каллорий тк нулевое время",
			workout: &model.Workouts{
				Distance:      1000,
				Duration:      0,
				ElevationGain: Ptr(100.0),
			},
			user: &model.User{
				WeightKg: 75,
			},
			wantCalories: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, tt.user)

			if tt.wantCalories == nil {
				assert.Nil(t, tt.workout.Calories)
			} else {
				require.NotNil(t, tt.workout.Calories)
				assert.Equal(t, tt.wantCalories, tt.workout.Calories)
			}
		})
	}
}

func TestCalculateDerivedMetrics_IF_TSS(t *testing.T) {
	tests := []struct {
		name    string
		workout *model.Workouts
		user    *model.User
		wantIF  *float64
		wantTSS *float64
	}{
		{
			name: "Нормальный бег по пороговому темпу (Threshold Pace)",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3000,
			},
			user: &model.User{
				ThresholdPace: 4.5,
			},
			wantIF:  Ptr(0.90),
			wantTSS: Ptr(67.5),
		},
		{
			name: "Fallback по ЧСС (когда ThresholdPace == 0)",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3600,
				AvgHR:    Ptr(150),
			},
			user: &model.User{
				ThresholdPace: 0,
				MaxHR:         180,
				RestingHR:     50,
			},
			wantIF:  Ptr(0.90),
			wantTSS: Ptr(81.0),
		},
		{
			name: "Учет RPE (субъективной нагрузки 8/10)",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3600,
				AvgHR:    Ptr(150),
				RPE:      Ptr(8),
			},
			user: &model.User{
				ThresholdPace: 0,
				MaxHR:         180,
			},
			wantIF:  Ptr(0.90),
			wantTSS: Ptr(80.5),
		},
		{
			name: "Клампинг IF сверху (сверхбыстрый спринт)",
			workout: &model.Workouts{
				Distance: 5,
				Duration: 900,
			},
			user: &model.User{
				ThresholdPace: 4.5,
			},
			wantIF:  Ptr(1.30),
			wantTSS: Ptr(42.3),
		},
		{
			name: "Нулевая длительность",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 0,
			},
			user: &model.User{
				ThresholdPace: 4.5,
			},
			wantIF:  nil,
			wantTSS: nil,
		},
		{
			name: "Нулевая дистанция",
			workout: &model.Workouts{
				Distance: 0,
				Duration: 900,
			},
			user: &model.User{
				ThresholdPace: 4.5,
			},
			wantIF:  nil,
			wantTSS: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, tt.user)

			if tt.wantIF == nil {
				assert.Nil(t, tt.workout.IntensityFactor)
			} else {
				require.NotNil(t, tt.workout.IntensityFactor)
				assert.Equal(t, tt.wantIF, tt.workout.IntensityFactor)
			}

			if tt.wantTSS == nil {
				assert.Nil(t, tt.workout.TrainingStressScore)
			} else {
				require.NotNil(t, tt.workout.TrainingStressScore)
				assert.Equal(t, tt.wantTSS, tt.workout.TrainingStressScore)
			}
		})
	}
}

func TestCalculateDerivedMetrics_Efficiency(t *testing.T) {
	tests := []struct {
		name           string
		workout        *model.Workouts
		wantEfficiency *float64
	}{
		{
			name: "Стандартный расчет",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3000,
				AvgHR:    Ptr(150),
			},
			wantEfficiency: Ptr(0.0333333),
		},
		{
			name: "Нулевой пульс",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3000,
				AvgHR:    Ptr(0),
			},
			wantEfficiency: nil,
		},
		{
			name: "Пульс не указан",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3000,
				AvgHR:    nil,
			},
			wantEfficiency: nil,
		},
		{
			name: "Нулевой темп (нулевая дистанция)",
			workout: &model.Workouts{
				Distance: 0,
				Duration: 3000,
				AvgHR:    Ptr(150),
			},
			wantEfficiency: nil,
		},
		{
			name: "Нулевой темп (нулевая длительность)",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 0,
				AvgHR:    Ptr(150),
			},
			wantEfficiency: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, &model.User{})

			if tt.wantEfficiency == nil {
				assert.Nil(t, tt.workout.Efficiency)
			} else {
				require.NotNil(t, tt.workout.Efficiency)
				assert.InDelta(t, *tt.wantEfficiency, *tt.workout.Efficiency, 1e-5)
			}
		})
	}
}

func TestCalculateDerivedMetrics_TE_PTF(t *testing.T) {
	tests := []struct {
		name      string
		workout   *model.Workouts
		user      *model.User
		wantTE    *float64
		wantFocus string
	}{
		{
			name: "Аэробная база по зонам (длительный легкий бег в Зоне 2)",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 3600,
				TimeInHrZone: []int{0,
					3600,
					0,
					0,
					0},
			},
			user: &model.User{
				RestingHR: 60,
			},
			wantTE:    Ptr(2.6),
			wantFocus: "Aerobic Endurance",
		},
		{
			name: "Интервальная темповая тренировка в Зоне 4",
			workout: &model.Workouts{
				Distance: 12,
				Duration: 3600,
				TimeInHrZone: []int{300,
					600,
					900,
					1800,
					0},
			},
			user: &model.User{
				RestingHR: 60,
			},
			wantTE:    Ptr(5.0),
			wantFocus: "VO2max / Threshold",
		},
		{
			name: "Fallback расчет по IF (когда зон пульса нет)",
			workout: &model.Workouts{
				Distance:     10,
				Duration:     3600,
				TimeInHrZone: nil,
			},
			user: &model.User{
				RestingHR:     60,
				ThresholdPace: 5.4,
			},
			wantTE:    Ptr(2.7),
			wantFocus: "Aerobic Endurance",
		},
		{
			name: "Нет зон и нет дистанции/скорости (эффект не может быть рассчитан)",
			workout: &model.Workouts{
				Distance:     0,
				Duration:     0,
				TimeInHrZone: nil, // Зон нет
			},
			user: &model.User{
				RestingHR: 60,
			},
			wantTE:    nil,
			wantFocus: "",
		},
		{
			name: "Нулевая длительность",
			workout: &model.Workouts{
				Distance: 10,
				Duration: 0,
				TimeInHrZone: []int{0,
					3600,
					0,
					0},
			},
			user: &model.User{
				RestingHR: 60,
			},
			wantTE:    nil,
			wantFocus: "",
		},
		{
			name: "Нет зон и нет данных пульса/порогового темпа для расчета IF",
			workout: &model.Workouts{
				Distance:     0,
				Duration:     3600,
				TimeInHrZone: nil, // Зон нет
				AvgHR:        nil, // Пульса нет
			},
			user: &model.User{
				ThresholdPace: 0, // Порогового темпа нет
			},
			wantTE:    nil,
			wantFocus: "",
		},
		{
			name: "Нулевая дистанция, но тренировка на тренажере по зонам пульса",
			workout: &model.Workouts{
				Distance:     0,
				Duration:     3600,
				TimeInHrZone: []int{0, 3600, 0, 0, 0},
			},
			user:      &model.User{},
			wantTE:    Ptr(2.6),
			wantFocus: "Aerobic Endurance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, tt.user)

			if tt.wantTE == nil {
				assert.Nil(t, tt.workout.AerobicTrainingEffect)
			} else {
				require.NotNil(t, tt.workout.AerobicTrainingEffect)
				assert.InDelta(t, *tt.wantTE, *tt.workout.AerobicTrainingEffect, 0.1)
			}

			if tt.wantFocus == "" {
				assert.Empty(t, tt.workout.PrimaryTrainingFocus)
			} else {
				assert.Equal(t, tt.wantFocus, tt.workout.PrimaryTrainingFocus)
			}
		})
	}
}

func TestCalculateDerivedMetrics_RecoveryTime(t *testing.T) {
	tests := []struct {
		name             string
		workout          *model.Workouts
		user             *model.User
		wantRecoveryTime *int
	}{
		{
			name: "Легкая восстановительная пробежка (IF < 0.80)",
			workout: &model.Workouts{
				Distance:                5.0,
				Duration:                1800, // Pace = 6.0
				AnaerobicTrainingEffect: Ptr(0.0),
			},
			user: &model.User{
				ThresholdPace: 4.5, // IF = 4.5 / 6.0 = 0.75
			},
			wantRecoveryTime: Ptr(384),
		},
		{
			name: "Тяжелая длинная темповая тренировка",
			workout: &model.Workouts{
				Distance:                15.0,
				Duration:                5400, // 90 мин, Pace = 6.0
				AnaerobicTrainingEffect: Ptr(1.1),
			},
			user: &model.User{
				ThresholdPace: 5.7, // IF = 5.7 / 6.0 = 0.95
			},
			wantRecoveryTime: Ptr(2122),
		},
		{
			name: "Минимальный лимит (клампинг снизу 6 часов = 360 мин)",
			workout: &model.Workouts{
				Distance:                1.0,
				Duration:                300, // Pace = 5.0
				AnaerobicTrainingEffect: Ptr(0.0),
			},
			user: &model.User{
				ThresholdPace: 3.6, // IF = 3.6 / 5.0 = 0.72
			},
			wantRecoveryTime: Ptr(360),
		},
		{
			name: "Метрики не рассчитаны (нулевое время)",
			workout: &model.Workouts{
				Distance: 0,
				Duration: 0,
			},
			user:             &model.User{},
			wantRecoveryTime: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculate.CalculateDerivedMetrics(tt.workout, tt.user)

			if tt.wantRecoveryTime == nil {
				assert.Nil(t, tt.workout.RecoveryTime)
			} else {
				require.NotNil(t, tt.workout.RecoveryTime)
				assert.Equal(t, *tt.wantRecoveryTime, *tt.workout.RecoveryTime)
			}
		})
	}
}

func TestCalculateTrainingEffect(t *testing.T) {
	tests := []struct {
		name          string
		zones         []int
		duration      int
		wantAerobic   *float64
		wantAnaerobic *float64
	}{
		{
			name:          "Пустые зоны пульса",
			zones:         nil,
			duration:      3600,
			wantAerobic:   nil,
			wantAnaerobic: nil,
		},
		{
			name:          "Нулевая длительность",
			zones:         []int{0, 1000, 0, 0, 0},
			duration:      0,
			wantAerobic:   nil,
			wantAnaerobic: nil,
		},
		{
			name:          "Только Зона 1 (восстановление, z2..z5 равны 0)",
			zones:         []int{3600, 0, 0, 0, 0},
			duration:      3600,
			wantAerobic:   nil,
			wantAnaerobic: nil,
		},
		{
			name:          "Тренировка в Зоне 2 (аэробная работа)",
			zones:         []int{0, 3600, 0, 0, 0},
			duration:      3600,
			wantAerobic:   Ptr(2.6),
			wantAnaerobic: Ptr(0.0),
		},
		{
			name:          "Тяжелая анаэробная работа в Зонах 4 и 5",
			zones:         []int{0, 0, 0, 1800, 1800},
			duration:      3600,
			wantAerobic:   Ptr(5.0),
			wantAnaerobic: Ptr(5.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAerobic, gotAnaerobic := calculate.CalculateTrainingEffect(tt.zones, tt.duration)

			if tt.wantAerobic == nil {
				assert.Nil(t, gotAerobic)
				assert.Nil(t, gotAnaerobic)
			} else {
				require.NotNil(t, gotAerobic)
				require.NotNil(t, gotAnaerobic)
				assert.InDelta(t, *tt.wantAerobic, *gotAerobic, 0.1)
				assert.InDelta(t, *tt.wantAnaerobic, *gotAnaerobic, 0.1)
			}
		})
	}
}

func TestDeterminePrimaryFocus(t *testing.T) {
	tests := []struct {
		name      string
		aerobic   float64
		anaerobic float64
		zones     []int
		want      string
	}{
		// Без зон (fallback по числам)
		{
			name:      "Без зон - высокая анаэробная нагрузка",
			aerobic:   2.5,
			anaerobic: 4.2,
			zones:     nil,
			want:      "Anaerobic Power",
		},
		{
			name:      "Без зон - умеренная анаэробная нагрузка",
			aerobic:   2.5,
			anaerobic: 3.1,
			zones:     nil,
			want:      "Anaerobic",
		},
		{
			name:      "Без зон - высокая аэробная нагрузка (VO2max)",
			aerobic:   4.5,
			anaerobic: 1.0,
			zones:     nil,
			want:      "VO2max / Threshold",
		},
		{
			name:      "Без зон - умеренная аэробная нагрузка (Aerobic Power)",
			aerobic:   3.3,
			anaerobic: 1.0,
			zones:     nil,
			want:      "Aerobic Power",
		},
		{
			name:      "Без зон - базовая аэробная нагрузка (Aerobic Endurance)",
			aerobic:   2.2,
			anaerobic: 0.5,
			zones:     nil,
			want:      "Aerobic Endurance",
		},
		// С зонами пульса
		{
			name:      "С зонами - анаэробный спринт (anaerobic > 4.0)",
			aerobic:   3.0,
			anaerobic: 4.5,
			zones:     []int{300, 300, 300, 600, 600},
			want:      "Anaerobic Power",
		},
		{
			name:      "С зонами - анаэробная работа (anaerobic > 2.8)",
			aerobic:   3.0,
			anaerobic: 2.9,
			zones:     []int{300, 300, 300, 600, 100},
			want:      "Anaerobic",
		},
		{
			name:      "С зонами - порог/VO2max (aerobic > 4.0)",
			aerobic:   4.8,
			anaerobic: 1.2,
			zones:     []int{300, 1500, 1500, 0, 0},
			want:      "VO2max / Threshold",
		},
		{
			name:      "С зонами - аэробная сила (aerobic > 3.2)",
			aerobic:   3.5,
			anaerobic: 0.5,
			zones:     []int{300, 1500, 1500, 0, 0},
			want:      "Aerobic Power",
		},
		{
			name:      "С зонами - базовая выносливость (default)",
			aerobic:   2.0,
			anaerobic: 0.2,
			zones:     []int{300, 1500, 500, 0, 0},
			want:      "Aerobic Endurance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculate.DeterminePrimaryFocus(tt.aerobic, tt.anaerobic, tt.zones)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper функция для приведения типа к указателю
func Ptr[T any](v T) *T {
	return &v
}
