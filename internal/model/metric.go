package model

type Metrics struct {
	ID            int64
	UserID        int64
	TotalWorkouts int
	TotalDistance float64
	TotalDuration int
	AvgPace       float64
	From          string
	To            string
	TotalCalories int64
}
