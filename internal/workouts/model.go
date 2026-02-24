package workouts

import "time"

/*
id SERIAL PRIMARY KEY,

	user_id INTEGER NOT NULL,
	distance DECIMAL(7,2) NOT NULL,
	duration INTEGER NOT NULL, -- в секундах
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
*/
type Workouts struct {
	ID           int
	UserID       int
	Date         time.Time
	Distance     float64
	Duration     int
	Pace         float64
	CreatedAt    time.Time
	TypeActivity string
}
