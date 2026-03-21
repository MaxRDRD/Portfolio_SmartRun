package model

import (
	"time"
)

type User struct {
	ID          int64
	Name        string
	Email       string
	Password    string
	CreatedAt   time.Time
	TOTPEnabled bool
	WeightKg    float64
	RestingHR   int
	MaxHR       int
	Gender      string
}
