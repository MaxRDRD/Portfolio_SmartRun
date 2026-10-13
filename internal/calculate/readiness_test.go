package calculate

import (
	"SmartRun/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateReadiness(t *testing.T) {
	tests := []struct {
		name               string
		workouts           []model.Workouts
		previousMetrics    *model.DailyMetric
		wantATL            float64
		wantCTL            float64
		wantTSB            float64
		wantFatigue        int
		wantReadiness      int
		wantRecommendation string
	}{
		{
			name:               "Новый пользователь без тренировок и без истории",
			workouts:           nil,
			previousMetrics:    nil,
			wantATL:            0.0,
			wantCTL:            0.0,
			wantTSB:            0.0,
			wantFatigue:        35,
			wantReadiness:      65,
			wantRecommendation: "Нормальная готовность: поддерживающая тренировка",
		},
		{
			name:            "Инерция с предыдущим днем (без новых тренировок сегодня)",
			workouts:        nil,
			previousMetrics: &model.DailyMetric{ATL: 50.0, CTL: 60.0},
			wantATL:         25.0, // 50 * 0.5
			wantCTL:         48.0, // 60 * 0.8
			wantTSB:         23.0, // 48 - 25
			wantFatigue:     7,    // 100 - (65 + 23*1.2 = 92.6 -> 93) = 7
			wantReadiness:   93,
			wantRecommendation: "Высокая готовность: можно делать качественную работу",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateReadiness(tt.workouts, tt.previousMetrics)

			require.NotNil(t, got)
			assert.InDelta(t, tt.wantATL, got.ATL, 0.1)
			assert.InDelta(t, tt.wantCTL, got.CTL, 0.1)
			assert.InDelta(t, tt.wantTSB, got.TSB, 0.1)
			assert.Equal(t, tt.wantFatigue, got.FatigueScore)
			assert.Equal(t, tt.wantReadiness, got.ReadinessScore)
			assert.Equal(t, tt.wantRecommendation, got.Recommendation)
		})
	}
}

func TestRollingStress(t *testing.T) {
	sevenWorkouts := make([]model.Workouts, 7)
	for i := 0; i < 7; i++ {
		sevenWorkouts[i] = model.Workouts{TrainingLoad: Ptr(70.0)}
	}

	tests := []struct {
		name               string
		workouts           []model.Workouts
		days               int
		thresholdPaceMinKm float64
		want               float64
	}{
		{
			name:               "Пустой срез тренировок",
			workouts:           nil,
			days:               7,
			thresholdPaceMinKm: 5.0,
			want:               0.0,
		},
		{
			name:               "Некорректное число дней (days = 0)",
			workouts:           sevenWorkouts,
			days:               0,
			thresholdPaceMinKm: 5.0,
			want:               0.0,
		},
		{
			name:               "Окно 7 дней (ATL) для 7 тренировок по 70 TSS (490/7 = 70.0)",
			workouts:           sevenWorkouts,
			days:               7,
			thresholdPaceMinKm: 5.0,
			want:               70.0,
		},
		{
			name:               "Окно 42 дня (CTL) для 7 тренировок по 70 TSS (490/42 ~ 11.67)",
			workouts:           sevenWorkouts,
			days:               42,
			thresholdPaceMinKm: 5.0,
			want:               490.0 / 42.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rollingStress(tt.workouts, tt.days, tt.thresholdPaceMinKm)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestWorkoutStressScore(t *testing.T) {
	tests := []struct {
		name               string
		workout            model.Workouts
		thresholdPaceMinKm float64
		want               float64
	}{
		{
			name: "Готовый TrainingLoad",
			workout: model.Workouts{
				TrainingLoad: Ptr(75.5),
			},
			thresholdPaceMinKm: 5.0,
			want:               75.5,
		},
		{
			name: "Нулевая длительность",
			workout: model.Workouts{
				Duration: 0,
			},
			thresholdPaceMinKm: 5.0,
			want:               0.0,
		},
		{
			name: "Расчет по темпу на пороге (1 час при IF=1.0 дает 100 TSS)",
			workout: model.Workouts{
				Distance: 12.0,
				Duration: 3600,
				Pace:     5.0,
			},
			thresholdPaceMinKm: 5.0,
			want:               100.0,
		},
		{
			name: "Поправка на тяжелый RPE 9 (+20%)",
			workout: model.Workouts{
				Distance: 12.0,
				Duration: 3600,
				Pace:     5.0,
				RPE:      Ptr(9),
			},
			thresholdPaceMinKm: 5.0,
			want:               120.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workoutStressScore(tt.workout, tt.thresholdPaceMinKm)
			assert.InDelta(t, tt.want, got, 0.1)
		})
	}
}

func TestEstimateThresholdPaceMinKm(t *testing.T) {
	tests := []struct {
		name     string
		workouts []model.Workouts
		want     float64
	}{
		{
			name:     "Дефолт 5.3 при пустом списке",
			workouts: nil,
			want:     5.3,
		},
		{
			name: "Расчет по устойчивым тренировкам 20-90 мин (топ 20% + 0.25)",
			workouts: []model.Workouts{
				{Duration: 30 * 60, Pace: 4.5},
				{Duration: 40 * 60, Pace: 4.6},
				{Duration: 50 * 60, Pace: 5.0},
				{Duration: 60 * 60, Pace: 5.5},
				{Duration: 10 * 60, Pace: 3.5}, // короткая (<20 мин) отсеивается
			},
			want: 4.75, // 4.5 + 0.25
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateThresholdPaceMinKm(tt.workouts)
			assert.InDelta(t, tt.want, got, 0.05)
		})
	}
}

func TestRecommendationByTSB(t *testing.T) {
	tests := []struct {
		name string
		tsb  float64
		want string
	}{
		{
			name: "Высокая готовность (TSB = 15)",
			tsb:  15.0,
			want: "Высокая готовность: можно делать качественную работу",
		},
		{
			name: "Граничная высокая готовность (TSB = 10)",
			tsb:  10.0,
			want: "Высокая готовность: можно делать качественную работу",
		},
		{
			name: "Нормальная готовность (TSB = 0)",
			tsb:  0.0,
			want: "Нормальная готовность: поддерживающая тренировка",
		},
		{
			name: "Граничная нормальная готовность (TSB = -10)",
			tsb:  -10.0,
			want: "Нормальная готовность: поддерживающая тренировка",
		},
		{
			name: "Низкая готовность (TSB = -10.1)",
			tsb:  -10.1,
			want: "Низкая готовность: лучше восстановление/легкий бег",
		},
		{
			name: "Глубокая усталость (TSB = -25)",
			tsb:  -25.0,
			want: "Низкая готовность: лучше восстановление/легкий бег",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, recommendationByTSB(tt.tsb))
		})
	}
}

func TestRecoveryBonusFromLatestWorkout(t *testing.T) {
	tests := []struct {
		name     string
		workouts []model.Workouts
		want     float64
	}{
		{
			name:     "Пустой список тренировок",
			workouts: nil,
			want:     0.0,
		},
		{
			name:     "RPE не указан (nil)",
			workouts: []model.Workouts{{RPE: nil}},
			want:     0.0,
		},
		{
			name:     "Легкая восстановительная пробежка (RPE = 3)",
			workouts: []model.Workouts{{RPE: Ptr(3)}},
			want:     5.0,
		},
		{
			name:     "Тяжелая изнуряющая тренировка (RPE = 9)",
			workouts: []model.Workouts{{RPE: Ptr(9)}},
			want:     -8.0,
		},
		{
			name:     "Умеренная тренировка (RPE = 6)",
			workouts: []model.Workouts{{RPE: Ptr(6)}},
			want:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, recoveryBonusFromLatestWorkout(tt.workouts))
		})
	}
}

func TestMultiFactorRecoveryAdjustment(t *testing.T) {
	tests := []struct {
		name            string
		workouts        []model.Workouts
		previousMetrics *model.DailyMetric
		want            float64
	}{
		{
			name:            "Пустые тренировки и история",
			workouts:        nil,
			previousMetrics: nil,
			want:            0.0,
		},
		{
			name: "Высокий стресс дня (-8) и отличный сон (+4)",
			workouts: []model.Workouts{
				{AvgStress: Ptr(80)},
			},
			previousMetrics: &model.DailyMetric{
				SleepScore: 90,
			},
			want: -4.0, // -8 + 4
		},
		{
			name: "Высокая вариабельность сердца HRV (+3)",
			workouts: []model.Workouts{
				{RmssdHrv: Ptr(70)},
				{RmssdHrv: Ptr(55)},
				{RmssdHrv: Ptr(55)},
			},
			previousMetrics: nil,
			want:            3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, multiFactorRecoveryAdjustment(tt.workouts, tt.previousMetrics))
		})
	}
}

func TestRecentMedianHRV(t *testing.T) {
	tests := []struct {
		name     string
		workouts []model.Workouts
		useRMSSD bool
		want     float64
	}{
		{
			name:     "Пустой список",
			workouts: nil,
			useRMSSD: true,
			want:     0.0,
		},
		{
			name: "Нечетное количество [50, 60, 70] -> медиана 60",
			workouts: []model.Workouts{
				{RmssdHrv: Ptr(70)},
				{RmssdHrv: Ptr(50)},
				{RmssdHrv: Ptr(60)},
			},
			useRMSSD: true,
			want:     60.0,
		},
		{
			name: "Четное количество [40, 50, 60, 70] -> медиана 55",
			workouts: []model.Workouts{
				{SdrrHrv: Ptr(40)},
				{SdrrHrv: Ptr(70)},
				{SdrrHrv: Ptr(50)},
				{SdrrHrv: Ptr(60)},
			},
			useRMSSD: false,
			want:     55.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, recentMedianHRV(tt.workouts, tt.useRMSSD))
		})
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		min  float64
		max  float64
		want float64
	}{
		{name: "Значение ниже минимума", v: 2.0, min: 5.0, max: 10.0, want: 5.0},
		{name: "Значение выше максимума", v: 12.0, min: 5.0, max: 10.0, want: 10.0},
		{name: "Значение внутри диапазона", v: 7.5, min: 5.0, max: 10.0, want: 7.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clampFloat(tt.v, tt.min, tt.max))
		})
	}
}
