package model

type Metrics struct {
	ID            int
	UserID        int
	TotalWorkouts int
	TotalDistance float64
	TotalDuration int
	AvgPace       float64
	From          string
	To            string
}
