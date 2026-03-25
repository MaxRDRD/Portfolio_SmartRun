package dto

import "time"

type CreateDailyMetricRequest struct {
	Date           string  `json:"date"`
	SleepScore     int     `json:"sleep_score"`
	BodyBatteryAvg float64 `json:"body_battery_avg"`
	Steps          int     `json:"steps"`
}

type UpdateDailyMetricRequest struct {
	ID             int       `json:"id" validate:"required"`
	UserID         int64     `json:"user_id"`
	Date           time.Time `json:"date"`
	CTL            float64   `json:"ctl"`
	ATL            float64   `json:"atl"`
	TSB            float64   `json:"tsb"`
	FatigueScore   int       `json:"fatigue_score"`
	ReadinessScore int       `json:"readiness_score"`
	BodyBatteryAvg float64   `json:"body_battery_avg"`
	Steps          int       `json:"steps"`
	TotalCalories  int       `json:"total_calories"`
	SleepScore     int       `json:"sleep_score"`
	StressAvg      int       `json:"stress_avg"`
	Recommendation string    `json:"recommendation"`
	StreakDays     int       `json:"streak_days"`
	Monotony       float64   `json:"monotony"` // показатель вариативности тренировок, чем выше - тем менее разнообразные тренировки
	Strain         *float64  `json:"strain"`   // показатель общей нагрузки, учитывающий и объем, и интенсивность
	UpdatedAt      time.Time `json:"updated_at"`
}

type DailyMetricResponse struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Date           string     `json:"date"`
	CTL            float64    `json:"ctl"`
	ATL            float64    `json:"atl"`
	TSB            float64    `json:"tsb"`
	FatigueScore   int        `json:"fatigue_score"`
	ReadinessScore int        `json:"readiness_score"`
	BodyBatteryAvg float64    `json:"body_battery_avg"`
	Steps          int        `json:"steps"`
	TotalCalories  int        `json:"total_calories"`
	SleepScore     int        `json:"sleep_score"`
	StressAvg      int        `json:"stress_avg"`
	Recommendation string     `json:"recommendation"`
	StreakDays     int        `json:"streak_days"`
	Monotony       float64    `json:"monotony"`
	Strain         *float64   `json:"strain"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}
