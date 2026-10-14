package response_test

import (
	"SmartRun/pkg/my_errors"
	"SmartRun/pkg/response"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "not found error",
			err:        my_errors.ErrWorkoutNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrapped not found error",
			err:        fmt.Errorf("service failed: %w", my_errors.ErrDailyMetricNotFound),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "conflict error",
			err:        my_errors.ErrUserAlreadyExists,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "unauthorized error",
			err:        my_errors.ErrInvalidToken,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "bad request error",
			err:        my_errors.ErrInvalidData,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown error defaults to 500",
			err:        errors.New("pgx: connection refused"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := my_errors.HTTPStatus(tt.err)
			assert.Equal(t, tt.wantStatus, got)
		})
	}
}

func TestSendError_InformationDisclosureProtection(t *testing.T) {
	// 500 ошибка не должна раскрывать внутренние детали БД клиенту
	dbErr := errors.New("pgx: syntax error at or near 'SELECT table secret_data'")

	req := httptest.NewRequest(http.MethodGet, "/api/workouts", nil)
	rec := httptest.NewRecorder()

	response.SendError(rec, req, dbErr)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body response.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "internal_error", body.Error)
	assert.Equal(t, "internal server error", body.Message)
	assert.NotContains(t, body.Message, "secret_data")
}

func TestSendError_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/workouts/123", nil)
	rec := httptest.NewRecorder()

	response.SendError(rec, req, my_errors.ErrWorkoutNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body response.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "not_found", body.Error)
	assert.Equal(t, "workout not found", body.Message)
}

func TestSendBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	response.SendBadRequest(rec, "invalid query parameter")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body response.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "bad_request", body.Error)
	assert.Equal(t, "invalid query parameter", body.Message)
}

func TestSendUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()

	response.SendUnauthorized(rec, "token expired")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body response.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "unauthorized", body.Error)
	assert.Equal(t, "token expired", body.Message)
}
