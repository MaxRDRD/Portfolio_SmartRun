package my_errors

import "errors"

var (
	// auth / user
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")

	// token / session
	ErrTokenNotFound    = errors.New("token not found")
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenAlreadyUsed = errors.New("token already used") // добавь для reset password

	// totp
	ErrSecretNotFound = errors.New("secret not found")

	// общие
	ErrInvalidData = errors.New("invalid data")
	ErrInternal    = errors.New("internal server error") // для 500 без утечек

	// metrics
	ErrMetricNotFound = errors.New("workout not found")

	//workouts
	ErrWorkoutNotFound      = errors.New("workout not found")
	ErrWorkoutAlreadyExists = errors.New("workout already exists")
	ErrQueryWorkoutHistory  = errors.New("error querying workout history")
	ErrWorkoutUpdate        = errors.New("error updating workout")

	//password_reser
	ErrPasswordResetHashNotFount = errors.New("password reset hash not found")

	//hrzones
	ErrHRZonesNotFound = errors.New("hrzones not found")

	//daily_metrics
	ErrDailyMetricNotFound      = errors.New("daily metric not found")
	ErrDailyMetricAlreadyExists = errors.New("daily metric already exists")
)
