package my_errors

import "errors"

var (
	ErrMetricNotFound = errors.New("workout not found")
	ErrInvalidData    = errors.New("invalid data")
)
