package dto

import "time"

type MetricsResponse struct {
	TotalWorkouts int     `json:"total_workouts"`
	TotalDistance float64 `json:"total_distance"`
	TotalDuration int     `json:"total_duration"`
	AvgPace       float64 `json:"avg_pace"`
	LongestRun    float64 `json:"longest_run"`
	FastestPace   float64 `json:"fastest_pace"`
	TotalCalories int64   `json:"total_calories"`
}

type MetricsFilter struct {
	UserID int64      `json:"user_id"`
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
}
