package metrics

import "errors"

var (
	ErrMetricNotFound     = errors.New("workout not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidData        = errors.New("invalid data")
)
