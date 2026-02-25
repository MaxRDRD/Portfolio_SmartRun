package metrics

import "time"

/*
type Metrics struct {
	ID            int
	UserID        int
	TotalWorkouts int
	TotalDistance float64
	TotalDuration int
	AvgPace       float64
}
*/

type MetricsResponse struct {
	TotalWorkouts int     `json:"user_id"`
	TotalDistance float64 `json:"user_id"`
	TotalDuration int     `json:"user_id"`
	AvgPace       float64 `json:"user_id"`
}

type MetricsFilter struct {
	UserID int        `json:"user_id"`
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
}
