package validate_test

import (
	"SmartRun/internal/model"
	"SmartRun/internal/validate"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckForAnomalies(t *testing.T) {
	// Вспомогательные указатели для полей int
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name        string
		workout     *model.Workouts
		wantAnomaly bool
	}{
		{
			name: "Тренировка без аномалий",
			workout: &model.Workouts{
				Pace:     5.5,
				Distance: 10.0,
				Duration: 3300,
				AvgHR:    intPtr(145),
				MaxHR:    intPtr(160),
			},
			wantAnomaly: false,
		},
		{
			name: "Слишком высокая скорость",
			workout: &model.Workouts{
				Pace:     1.5,
				Distance: 10.0,
				Duration: 1800,
				AvgHR:    intPtr(145),
				MaxHR:    intPtr(160),
			},
			wantAnomaly: true,
		},
		{
			name: "Слишком медленный темп",
			workout: &model.Workouts{
				Pace:     31.0,
				Distance: 10.0,
				Duration: 6000,
				AvgHR:    intPtr(145),
				MaxHR:    intPtr(160),
			},
			wantAnomaly: true,
		},
		{
			name: "Длинная корректная дистанция",
			workout: &model.Workouts{
				Pace:     4.0,
				Distance: 20.0, // 20 км
				Duration: 4800, // 80 минут (20 км за 80 минут = 4:00/км, что быстро, но возможно)
				AvgHR:    intPtr(145),
				MaxHR:    intPtr(160),
			},
			wantAnomaly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			gotAnomaly, reason := validate.CheckForAnomalies(tt.workout)
			is.Equal(tt.wantAnomaly, gotAnomaly, reason)
			if tt.wantAnomaly {
				is.NotEmpty(reason)
			} else {
				is.Empty(reason)
			}
		})
	}
}
