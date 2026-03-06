package model

import "time"

type Workouts struct {
	ID           int
	UserID       int
	Date         time.Time
	Distance     float64
	Duration     int
	Pace         float64
	CreatedAt    time.Time
	TypeActivity string
	Calories     int //задел на будущее, пока не реализовано
}
