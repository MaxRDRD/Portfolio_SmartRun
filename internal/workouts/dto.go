package workouts

/*
type Workouts struct {
	ID           int
	userID       int
	Date         time.Time
	Distance     float32
	Duration     int
	Pace         int
	CreatedAt    time.Time
	TypeActivity string
}
*/

type WorkoutsResponse struct {
	ID           int     `json:"id"`
	Date         string  `json:"date" validate:"required"`
	TypeActivity string  `json:"type_activity"`
	Distance     float64 `json:"distance" validate:"required"`
	Duration     int     `json:"duration" validate:"required"`
}

type CreateRequest struct {
	Date         string  `json:"date" validate:"required"`
	Distance     float64 `json:"distance" validate:"required"`
	Duration     int     `json:"duration" validate:"required"`
	TypeActivity string  `json:"type_activity"`
}

type UpdateRequest struct {
	Date         string  `json:"date"`
	Distance     float64 `json:"distance"`
	Duration     int     `json:"duration"`
	TypeActivity string  `json:"type_activity"`
}
