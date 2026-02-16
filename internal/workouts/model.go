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
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Distance  float32   `json:"distance" validate:"required"`
	Duration  int       `json:"duration" validate:"required"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
}
