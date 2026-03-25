package dto

import "time"

type CreateMetricsRequest struct {
	TotalWorkouts int     `json:"total_workouts"`
	TotalDistance float64 `json:"total_distance"`
	TotalDuration int     `json:"total_duration"`
	AvgPace       float64 `json:"avg_pace"`
	TotalCalories int64   `json:"total_calories"`
	From          string  `json:"from"`
	To            string  `json:"to"`
}

type UpdateMetricsRequest struct {
	TotalWorkouts int     `json:"total_workouts"`
	TotalDistance float64 `json:"total_distance"`
	TotalDuration int     `json:"total_duration"`
	AvgPace       float64 `json:"avg_pace"`
	TotalCalories int64   `json:"total_calories"`
	From          string  `json:"from"`
	To            string  `json:"to"`
}

type MetricsResponse struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	TotalWorkouts int     `json:"total_workouts"`
	TotalDistance float64 `json:"total_distance"`
	TotalDuration int     `json:"total_duration"`
	AvgPace       float64 `json:"avg_pace"`
	LongestRun    float64 `json:"longest_run"`
	FastestPace   float64 `json:"fastest_pace"`
	TotalCalories int64   `json:"total_calories"`
	From          string  `json:"from,omitempty"`
	To            string  `json:"to,omitempty"`
}

type MetricsFilter struct {
	ID     int64      `json:"id,omitempty"`
	UserID int64      `json:"user_id"`
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
}
