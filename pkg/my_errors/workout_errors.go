package my_errors

import "errors"

var (
	ErrWorkoutNotFound      = errors.New("workout not found")
	ErrWorkoutAlreadyExists = errors.New("workout already exists")
)
