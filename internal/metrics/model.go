package metrics

/*
id SERIAL PRIMARY KEY,

	workout_id INTEGER NOT NULL,
	pace INTEGER NOT NULL,
	time_run INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
*/
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
