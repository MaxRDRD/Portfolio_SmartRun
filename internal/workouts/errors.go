package workouts

import "errors"

var (
	ErrWorkoutNotFound      = errors.New("workout not found")
	ErrWorkoutAlreadyExists = errors.New("workout already exists")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidData          = errors.New("invalid data")
)
