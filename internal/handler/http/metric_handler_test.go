package http_test

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	my_http "SmartRun/internal/handler/http"
	"SmartRun/internal/handler/http/mocks"
	"SmartRun/internal/model"
	"SmartRun/pkg/my_errors"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMetricHandler_CreateMetrics(t *testing.T) {
	validBody := `{"total_workouts": 5, "total_distance": 50.0, "total_duration": 18000, "avg_pace": 6.0, "total_calories": 2500, "from": "2026-01-01", "to": "2026-01-07"}`

	tests := []struct {
		name           string
		authUserID     *int64
		body           string
		mockBehavior   func(m *mocks.MetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			body:           validBody,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON",
			authUserID:     Ptr(int64(12)),
			body:           `{invalid_json}`,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:        "400 Bad Request on service error",
			authUserID:  Ptr(int64(12)),
			body:        validBody,
			mockBehavior: func(m *mocks.MetricService) {
				m.On("CreateMetrics", mock.Anything, mock.MatchedBy(func(m model.Metrics) bool {
					return m.UserID == 12 && m.TotalWorkouts == 5 && m.TotalDistance == 50.0
				})).Return(nil, my_errors.ErrInvalidData).Once()
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid data")
			},
		},
		{
			name:        "201 Created Success",
			authUserID:  Ptr(int64(12)),
			body:        validBody,
			mockBehavior: func(m *mocks.MetricService) {
				created := &model.Metrics{
					ID:            101,
					UserID:        12,
					TotalWorkouts: 5,
					TotalDistance: 50.0,
					TotalDuration: 18000,
					AvgPace:       6.0,
					TotalCalories: 2500,
					From:          "2026-01-01",
					To:            "2026-01-07",
				}
				m.On("CreateMetrics", mock.Anything, mock.AnythingOfType("model.Metrics")).
					Return(created, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.MetricsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(101), res.ID)
				assert.Equal(t, int64(12), res.UserID)
				assert.Equal(t, 5, res.TotalWorkouts)
				assert.Equal(t, 50.0, res.TotalDistance)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.MetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/metrics", strings.NewReader(tt.body))
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.CreateMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestMetricHandler_GetMetrics(t *testing.T) {
	tests := []struct {
		name           string
		queryURL       string
		authUserID     *int64
		mockBehavior   func(m *mocks.MetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			queryURL:       "/api/metrics",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid 'from' date format",
			queryURL:       "/api/metrics?from=invalid-date",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid 'from' date format")
			},
		},
		{
			name:           "400 Bad Request on invalid 'to' date format",
			queryURL:       "/api/metrics?from=2026-01-01&to=bad-date",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid 'to' date format")
			},
		},
		{
			name:       "404 Not Found on service error",
			queryURL:   "/api/metrics?from=2026-01-01&to=2026-01-07",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				m.On("GetMetrics", mock.Anything, mock.MatchedBy(func(f dto.MetricsFilter) bool {
					return f.UserID == 12 && f.From != nil && f.To != nil
				})).Return(nil, my_errors.ErrMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "metric not found")
			},
		},
		{
			name:       "200 Success with valid dates",
			queryURL:   "/api/metrics?from=2026-01-01&to=2026-01-07",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				expected := &model.Metrics{
					ID:            101,
					UserID:        12,
					TotalWorkouts: 4,
					TotalDistance: 42.0,
					TotalDuration: 14400,
					AvgPace:       5.7,
					TotalCalories: 2100,
					From:          "2026-01-01",
					To:            "2026-01-07",
				}
				m.On("GetMetrics", mock.Anything, mock.MatchedBy(func(f dto.MetricsFilter) bool {
					return f.UserID == 12 && f.From != nil && f.To != nil
				})).Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.MetricsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(101), res.ID)
				assert.Equal(t, 4, res.TotalWorkouts)
				assert.Equal(t, 42.0, res.TotalDistance)
			},
		},
		{
			name:       "200 Success without dates",
			queryURL:   "/api/metrics",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				expected := &model.Metrics{
					ID:            102,
					UserID:        12,
					TotalWorkouts: 10,
					TotalDistance: 100.0,
				}
				m.On("GetMetrics", mock.Anything, mock.MatchedBy(func(f dto.MetricsFilter) bool {
					return f.UserID == 12 && f.From == nil && f.To == nil
				})).Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.MetricsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(102), res.ID)
				assert.Equal(t, 10, res.TotalWorkouts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.MetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, tt.queryURL, nil)
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestMetricHandler_GetStoredMetrics(t *testing.T) {
	tests := []struct {
		name           string
		authUserID     *int64
		mockBehavior   func(m *mocks.MetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:       "404 Not Found on service error",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				m.On("GetAllMetrics", mock.Anything, dto.MetricsFilter{UserID: 12}).
					Return(nil, my_errors.ErrMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "metric not found")
			},
		},
		{
			name:       "200 Success",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				expected := &model.Metrics{
					ID:            201,
					UserID:        12,
					TotalWorkouts: 25,
					TotalDistance: 250.0,
					TotalDuration: 90000,
					AvgPace:       5.5,
					TotalCalories: 15000,
				}
				m.On("GetAllMetrics", mock.Anything, dto.MetricsFilter{UserID: 12}).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.MetricsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(201), res.ID)
				assert.Equal(t, 25, res.TotalWorkouts)
				assert.Equal(t, 250.0, res.TotalDistance)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.MetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, "/api/metrics/stored", nil)
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetStoredMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestMetricHandler_UpdateMetrics(t *testing.T) {
	validBody := `{"total_workouts": 6, "total_distance": 60.0, "total_duration": 21600, "avg_pace": 5.9, "total_calories": 3000, "from": "2026-01-01", "to": "2026-01-08"}`

	tests := []struct {
		name           string
		paramID        string
		authUserID     *int64
		body           string
		mockBehavior   func(m *mocks.MetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			paramID:        "1",
			authUserID:     nil,
			body:           validBody,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON",
			paramID:        "1",
			authUserID:     Ptr(int64(12)),
			body:           `{invalid_json}`,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:           "400 Bad Request on invalid 'from' date format",
			paramID:        "1",
			authUserID:     Ptr(int64(12)),
			body:           `{"from": "not_a_date"}`,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid 'from' date format")
			},
		},
		{
			name:           "400 Bad Request on invalid 'to' date format",
			paramID:        "1",
			authUserID:     Ptr(int64(12)),
			body:           `{"from": "2026-01-01", "to": "not_a_date"}`,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid 'to' date format")
			},
		},
		{
			name:           "400 Bad Request on invalid id",
			paramID:        "abc",
			authUserID:     Ptr(int64(12)),
			body:           validBody,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:        "404 Not Found on service error",
			paramID:     "999",
			authUserID:  Ptr(int64(12)),
			body:        validBody,
			mockBehavior: func(m *mocks.MetricService) {
				m.On("UpdateMetrics", mock.Anything, mock.MatchedBy(func(m model.Metrics) bool {
					return m.ID == 999 && m.UserID == 12
				})).Return(nil, my_errors.ErrMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "metric not found")
			},
		},
		{
			name:        "200 Success",
			paramID:     "101",
			authUserID:  Ptr(int64(12)),
			body:        validBody,
			mockBehavior: func(m *mocks.MetricService) {
				updated := &model.Metrics{
					ID:            101,
					UserID:        12,
					TotalWorkouts: 6,
					TotalDistance: 60.0,
					TotalDuration: 21600,
					AvgPace:       5.9,
					TotalCalories: 3000,
					From:          "2026-01-01",
					To:            "2026-01-08",
				}
				m.On("UpdateMetrics", mock.Anything, mock.MatchedBy(func(m model.Metrics) bool {
					return m.ID == 101 && m.UserID == 12 && m.TotalWorkouts == 6
				})).Return(updated, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.MetricsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(101), res.ID)
				assert.Equal(t, 6, res.TotalWorkouts)
				assert.Equal(t, 60.0, res.TotalDistance)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.MetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPut, "/api/metrics/"+tt.paramID, strings.NewReader(tt.body))

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.UpdateMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestMetricHandler_DeleteMetrics(t *testing.T) {
	tests := []struct {
		name           string
		paramID        string
		authUserID     *int64
		mockBehavior   func(m *mocks.MetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			paramID:        "1",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid id",
			paramID:        "abc",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.MetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:       "404 Not Found on service error",
			paramID:    "999",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				m.On("DeleteMetrics", mock.Anything, int64(999), int64(12)).
					Return(my_errors.ErrMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "metric not found")
			},
		},
		{
			name:       "204 Success",
			paramID:    "101",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.MetricService) {
				m.On("DeleteMetrics", mock.Anything, int64(101), int64(12)).
					Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Empty(t, rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.MetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodDelete, "/api/metrics/"+tt.paramID, nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.DeleteMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestToMetricsResponse_Nil(t *testing.T) {
	res := my_http.ExportToMetricsResponse(nil)
	assert.Equal(t, int64(0), res.ID)
}

