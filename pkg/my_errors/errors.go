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
	ErrTokenAlreadyUsed = errors.New("token already used")

	// totp
	ErrSecretNotFound = errors.New("secret not found")

	// общие
	ErrInvalidData = errors.New("invalid data")
	ErrInternal    = errors.New("internal server error")

	// metrics
	ErrMetricNotFound = errors.New("metric not found")

	//workouts
	ErrWorkoutNotFound      = errors.New("workout not found")
	ErrWorkoutAlreadyExists = errors.New("workout already exists")
	ErrQueryWorkoutHistory  = errors.New("error querying workout history")
	ErrWorkoutUpdate        = errors.New("error updating workout")

	//password_reser
	ErrPasswordResetHashNotFound = errors.New("password reset hash not found")

	//hrzones
	ErrHRZonesNotFound = errors.New("hrzones not found")

	//daily_metrics
	ErrDailyMetricNotFound      = errors.New("daily metric not found")
	ErrDailyMetricAlreadyExists = errors.New("daily metric already exists")
)

func HTTPStatus(err error) int {
	//404
	NotFound := []error{ErrUserNotFound, ErrWorkoutNotFound, ErrDailyMetricNotFound, ErrMetricNotFound, ErrSecretNotFound, ErrPasswordResetHashNotFound}
	for _, target := range NotFound {
		if errors.Is(err, target) {
			return 404
		}
	}
	//409
	Conflict := []error{ErrUserAlreadyExists, ErrWorkoutAlreadyExists, ErrDailyMetricAlreadyExists}
	for _, target := range Conflict {
		if errors.Is(err, target) {
			return 409
		}
	}
	//401
	Unauthorized := []error{ErrInvalidToken, ErrTokenNotFound, ErrTokenAlreadyUsed, ErrInvalidCredentials}
	for _, target := range Unauthorized {
		if errors.Is(err, target) {
			return 401
		}
	}
	//400
	BadRequest := []error{ErrInvalidData}
	for _, target := range BadRequest {
		if errors.Is(err, target) {
			return 400
		}
	}
	return 500
}
