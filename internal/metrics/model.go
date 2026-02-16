package metrics

import "time"

/*
id SERIAL PRIMARY KEY,

	workout_id INTEGER NOT NULL,
	pace INTEGER NOT NULL,
	time_run INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
*/
type Metrics struct {
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Pace      int       `json:"pace" validate:"required,max=1000000"`
	TimeRun   int       `json:"time_run" validate:"required,min=60"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
}
