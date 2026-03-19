package dto

import "time"

type WorkoutsResponse struct {
	ID           int64   `json:"id"`
	Date         string  `json:"date"`
	TypeActivity string  `json:"type_activity"`
	Distance     float64 `json:"distance"`
	Duration     int     `json:"duration"`
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

type WorkoutFilter struct {
	UserID int64      `json:"user_id" validate:"required"`
	Type   string     `json:"type"`
	From   *time.Time `json:"from" validate:"omitempty,gte=0"`
	To     *time.Time `json:"to" validate:"omitempty,gte=0"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}
