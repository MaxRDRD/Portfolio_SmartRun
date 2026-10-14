package calculate

import (
	"SmartRun/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateDailyMetrics(t *testing.T) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	tests := []struct {
		name            string
		workouts        []model.Workouts
		previousMetrics *model.DailyMetric
		targetDate      time.Time
		wantUserID      int64
		wantStreak      int
		wantCalories    int
		wantStressAvg   int
		wantSleepScore  int
		wantBodyBattery float64
	}{
		{
			name:            "День отдыха (0 тренировок, сохранение сна и батареи)",
			workouts:        nil,
			previousMetrics: &model.DailyMetric{UserID: 42, StreakDays: 5, SleepScore: 84, BodyBatteryAvg: 75.0, ATL: 40.0, CTL: 50.0},
			targetDate:      today,
			wantUserID:      42,
			wantStreak:      0,
			wantCalories:    0,
			wantStressAvg:   0,
			wantSleepScore:  84,
			wantBodyBattery: 75.0,
		},
		{
			name: "Тренировка была вчера, сегодня день отдыха (сброс стрика)",
			workouts: []model.Workouts{
				{UserID: 42, Date: yesterday, Calories: Ptr(500)},
			},
			previousMetrics: &model.DailyMetric{UserID: 42, StreakDays: 3, SleepScore: 78},
			targetDate:      today,
			wantUserID:      42,
			wantStreak:      0,
			wantCalories:    0,
			wantStressAvg:   0,
			wantSleepScore:  78,
			wantBodyBattery: 0.0,
		},
		{
			name: "Первый день пользователя со спортом (prev == nil)",
			workouts: []model.Workouts{
				{UserID: 101, Date: today, Calories: Ptr(650), AvgStress: Ptr(45), TrainingLoad: Ptr(75.0)},
			},
			previousMetrics: nil,
			targetDate:      today,
			wantUserID:      101,
			wantStreak:      1,
			wantCalories:    650,
			wantStressAvg:   45,
			wantSleepScore:  0,
			wantBodyBattery: 0.0,
		},
		{
			name: "Две тренировки в один день (суммирование калорий и средний стресс)",
			workouts: []model.Workouts{
				{UserID: 7, Date: today.Add(14 * time.Hour), Calories: Ptr(400), AvgStress: Ptr(30), TrainingLoad: Ptr(40.0)},
				{UserID: 7, Date: today.Add(8 * time.Hour), Calories: Ptr(350), AvgStress: Ptr(50), TrainingLoad: Ptr(35.0)},
			},
			previousMetrics: &model.DailyMetric{UserID: 7, StreakDays: 2},
			targetDate:      today,
			wantUserID:      7,
			wantStreak:      3,
			wantCalories:    750,
			wantStressAvg:   40,
			wantSleepScore:  0,
			wantBodyBattery: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDailyMetrics(tt.workouts, tt.previousMetrics, tt.targetDate)

			require.NotNil(t, got)
			assert.Equal(t, tt.wantUserID, got.UserID)
			assert.Equal(t, tt.targetDate, got.Date)
			assert.Equal(t, tt.wantStreak, got.StreakDays)
			assert.Equal(t, tt.wantCalories, got.TotalCalories)
			assert.Equal(t, tt.wantStressAvg, got.StressAvg)

			if tt.wantSleepScore > 0 {
				assert.Equal(t, tt.wantSleepScore, got.SleepScore)
			}
			if tt.wantBodyBattery > 0 {
				assert.Equal(t, tt.wantBodyBattery, got.BodyBatteryAvg)
			}
		})
	}
}

func TestCalcMonotonyAndStrain(t *testing.T) {
	now := time.Now().UTC()
	anchor := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		workouts      []model.Workouts
		anchorDate    time.Time
		wantMonotony  float64
		wantHasStrain bool
		wantStrain    *float64
	}{
		{
			name:          "Пустая неделя без тренировок",
			workouts:      nil,
			anchorDate:    anchor,
			wantMonotony:  0.0,
			wantHasStrain: false,
			wantStrain:    nil,
		},
		{
			name: "Максимальная монотонность - одинаковый бег каждый день (7 дней)",
			workouts: func() []model.Workouts {
				list := make([]model.Workouts, 7)
				for i := 0; i < 7; i++ {
					list[i] = model.Workouts{
						Date:         anchor.AddDate(0, 0, -i),
						TrainingLoad: Ptr(50.0),
					}
				}
				return list
			}(),
			anchorDate:    anchor,
			wantMonotony:  10.0,
			wantHasStrain: true,
			wantStrain:    Ptr(3500.0),
		},
		{
			name: "Вариативная неделя с днями отдыха (чередование нагрузки)",
			workouts: []model.Workouts{
				{Date: anchor.AddDate(0, 0, -1), TrainingLoad: Ptr(60.0)},
				{Date: anchor.AddDate(0, 0, -3), TrainingLoad: Ptr(80.0)},
				{Date: anchor.AddDate(0, 0, -5), TrainingLoad: Ptr(100.0)},
			},
			anchorDate:    anchor,
			wantMonotony:  0.85,
			wantHasStrain: true,
			wantStrain:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monotony, strain := calcMonotonyAndStrain(tt.workouts, tt.anchorDate)

			if !tt.wantHasStrain {
				assert.Equal(t, tt.wantMonotony, monotony)
				assert.Nil(t, strain)
			} else {
				require.NotNil(t, strain)
				if tt.wantStrain != nil {
					assert.InDelta(t, *tt.wantStrain, *strain, 0.1)
					assert.Equal(t, tt.wantMonotony, monotony)
				} else {
					assert.True(t, monotony > 0 && monotony < 2.0)
					assert.True(t, *strain > 0)
				}
			}
		})
	}
}

func TestSumCaloriesByDate(t *testing.T) {
	today := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	workouts := []model.Workouts{
		{Date: today.Add(10 * time.Hour), Calories: Ptr(300)},
		{Date: today.Add(6 * time.Hour), Calories: Ptr(250)},
		{Date: today.Add(2 * time.Hour), Calories: nil},
		{Date: yesterday.Add(18 * time.Hour), Calories: Ptr(400)},
	}

	tests := []struct {
		name       string
		workouts   []model.Workouts
		targetDate time.Time
		want       int
	}{
		{
			name:       "Две тренировки сегодня (300 + 250)",
			workouts:   workouts,
			targetDate: today,
			want:       550,
		},
		{
			name:       "Одна тренировка вчера",
			workouts:   workouts,
			targetDate: yesterday,
			want:       400,
		},
		{
			name:       "Дата без тренировок",
			workouts:   workouts,
			targetDate: today.AddDate(0, 0, -5),
			want:       0,
		},
		{
			name:       "Пустой срез тренировок",
			workouts:   nil,
			targetDate: today,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sumCaloriesByDate(tt.workouts, tt.targetDate))
		})
	}
}

func TestAvgStressByDate(t *testing.T) {
	today := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		workouts   []model.Workouts
		targetDate time.Time
		want       int
	}{
		{
			name: "Нет данных стресса (AvgStress == nil)",
			workouts: []model.Workouts{
				{Date: today, AvgStress: nil},
			},
			targetDate: today,
			want:       0,
		},
		{
			name: "Несколько тренировок с разным стрессом (округление среднего)",
			workouts: []model.Workouts{
				{Date: today, AvgStress: Ptr(40)},
				{Date: today, AvgStress: Ptr(60)},
			},
			targetDate: today,
			want:       50,
		},
		{
			name:       "Пустой срез тренировок",
			workouts:   nil,
			targetDate: today,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, avgStressByDate(tt.workouts, tt.targetDate))
		})
	}
}

func TestCalcStreakDaysIncremental(t *testing.T) {
	tests := []struct {
		name            string
		todayHasWorkout bool
		previousMetric  *model.DailyMetric
		want            int
	}{
		{
			name:            "Первый день с тренировкой (prev == nil)",
			todayHasWorkout: true,
			previousMetric:  nil,
			want:            1,
		},
		{
			name:            "Первый день без тренировки (prev == nil)",
			todayHasWorkout: false,
			previousMetric:  nil,
			want:            0,
		},
		{
			name:            "Продолжение стрика (prev = 7, сегодня бегал)",
			todayHasWorkout: true,
			previousMetric:  &model.DailyMetric{StreakDays: 7},
			want:            8,
		},
		{
			name:            "Сброс стрика (prev = 7, сегодня отдых)",
			todayHasWorkout: false,
			previousMetric:  &model.DailyMetric{StreakDays: 7},
			want:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, calcStreakDaysIncremental(tt.todayHasWorkout, tt.previousMetric))
		})
	}
}

func Ptr[T any](v T) *T {
	return &v
}
