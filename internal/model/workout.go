package model

import "time"

type Workouts struct {
	ID           int64
	UserID       int64
	Date         time.Time
	Distance     float64
	Duration     int
	Pace         float64
	CreatedAt    time.Time
	TypeActivity string
	Calories     int //задел на будущее, пока не реализовано
}

type Split struct {
	WorkoutID int64
	Distance  float64
	Duration  int
	Pace      float64
}

type HeartRate struct {
	WorkoutID int64
	AvgHR     int
	MaxHR     int
}
