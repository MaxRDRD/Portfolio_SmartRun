package http_test

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	my_http "SmartRun/internal/handler/http"
	"SmartRun/internal/handler/http/mocks"
	"SmartRun/internal/model"
	"SmartRun/pkg/apperrors"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDailyMetricHandler_CreateDailyMetric(t *testing.T) {
	validBodyWithDate := `{"date": "2026-02-15", "sleep_score": 85, "sleep_hours": 7.5, "body_battery_avg": 70, "steps": 10500}`
	validBodyNoDate := `{"sleep_score": 90, "sleep_hours": 8.0, "body_battery_avg": 80, "steps": 12000}`

	tests := []struct {
		name           string
		authUserID     *int64
		body           string
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			body:           validBodyWithDate,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON body",
			authUserID:     Ptr(int64(12)),
			body:           `{invalid_json}`,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:           "400 Bad Request on invalid date format",
			authUserID:     Ptr(int64(12)),
			body:           `{"date": "not_a_date"}`,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid date format, expected YYYY-MM-DD")
			},
		},
		{
			name:       "409 Conflict on duplicate metric",
			authUserID: Ptr(int64(12)),
			body:       validBodyWithDate,
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("CreateDailyMetric", mock.Anything, mock.MatchedBy(func(m model.DailyMetric) bool {
					return m.UserID == 12 && m.SleepScore == 85
				})).Return(nil, apperrors.ErrDailyMetricAlreadyExists).Once()
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "daily metric already exists")
			},
		},
		{
			name:       "201 Created Success with explicit date",
			authUserID: Ptr(int64(12)),
			body:       validBodyWithDate,
			mockBehavior: func(m *mocks.DailyMetricService) {
				expectedDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
				created := &model.DailyMetric{
					ID:             501,
					UserID:         12,
					Date:           expectedDate,
					CTL:            45.0,
					ATL:            50.0,
					TSB:            -5.0,
					SleepScore:     85,
					SleepHours:     7.5,
					BodyBatteryAvg: 70.0,
					Steps:          10500,
					Strain:         Ptr(12.5),
					UpdatedAt:      time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC),
				}
				m.On("CreateDailyMetric", mock.Anything, mock.MatchedBy(func(m model.DailyMetric) bool {
					return m.UserID == 12 && m.Date.Equal(expectedDate)
				})).Return(created, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(501), res.ID)
				assert.Equal(t, "2026-02-15", res.Date)
				assert.Equal(t, 45.0, res.CTL)
				assert.Equal(t, 85, res.SleepScore)
				assert.Equal(t, 7.5, res.SleepHours)
				assert.Equal(t, 70.0, res.BodyBatteryAvg)
				assert.Equal(t, 10500, res.Steps)
				assert.NotNil(t, res.Strain)
				assert.Equal(t, 12.5, *res.Strain)
				assert.NotNil(t, res.UpdatedAt)
			},
		},
		{
			name:       "201 Created Success without date (uses now)",
			authUserID: Ptr(int64(12)),
			body:       validBodyNoDate,
			mockBehavior: func(m *mocks.DailyMetricService) {
				created := &model.DailyMetric{
					ID:             502,
					UserID:         12,
					Date:           time.Now().UTC(),
					SleepScore:     90,
					SleepHours:     8.0,
					BodyBatteryAvg: 80.0,
					Steps:          12000,
				}
				m.On("CreateDailyMetric", mock.Anything, mock.MatchedBy(func(m model.DailyMetric) bool {
					return m.UserID == 12 && m.SleepScore == 90
				})).Return(created, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(502), res.ID)
				assert.Equal(t, 90, res.SleepScore)
				assert.Nil(t, res.UpdatedAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/daily-metrics", strings.NewReader(tt.body))
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.CreateDailyMetric(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDailyMetricHandler_GetDailyMetrics(t *testing.T) {
	tests := []struct {
		name           string
		authUserID     *int64
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:       "404 Not Found on service error",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("GetAllDailyMetrics", mock.Anything, int64(12)).
					Return(nil, apperrors.ErrDailyMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "daily metric not found")
			},
		},
		{
			name:       "200 Success with list of metrics",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				d1 := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
				d2 := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
				list := []model.DailyMetric{
					{
						ID:             501,
						UserID:         12,
						Date:           d1,
						CTL:            40.0,
						ATL:            45.0,
						TSB:            -5.0,
						ReadinessScore: 75,
					},
					{
						ID:             502,
						UserID:         12,
						Date:           d2,
						CTL:            42.0,
						ATL:            44.0,
						TSB:            -2.0,
						ReadinessScore: 80,
					},
				}
				m.On("GetAllDailyMetrics", mock.Anything, int64(12)).
					Return(list, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res []dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				require.Len(t, res, 2)
				assert.Equal(t, int64(501), res[0].ID)
				assert.Equal(t, "2026-02-14", res[0].Date)
				assert.Equal(t, 75, res[0].ReadinessScore)
				assert.Equal(t, int64(502), res[1].ID)
				assert.Equal(t, "2026-02-15", res[1].Date)
				assert.Equal(t, 80, res[1].ReadinessScore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, "/api/daily-metrics", nil)
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetDailyMetrics(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDailyMetricHandler_GetDailyMetricByID(t *testing.T) {
	tests := []struct {
		name           string
		paramID        string
		queryURL       string
		authUserID     *int64
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "400 Bad Request on invalid ID",
			paramID:        "invalid_id",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:           "401 Unauthorized",
			paramID:        "10",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:       "404 Not Found on service error",
			paramID:    "999",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("GetDailyMetricByID", mock.Anything, int64(999), int64(12)).
					Return(nil, apperrors.ErrDailyMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "daily metric not found")
			},
		},
		{
			name:       "200 Success via URLParam",
			paramID:    "501",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				metric := &model.DailyMetric{
					ID:             501,
					UserID:         12,
					Date:           time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
					CTL:            50.0,
					ReadinessScore: 88,
				}
				m.On("GetDailyMetricByID", mock.Anything, int64(501), int64(12)).
					Return(metric, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(501), res.ID)
				assert.Equal(t, 50.0, res.CTL)
				assert.Equal(t, 88, res.ReadinessScore)
			},
		},
		{
			name:       "200 Success via QueryParam fallback",
			paramID:    "", // no URLParam, fallback to ?id=502
			queryURL:   "/api/daily-metrics?id=502",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				metric := &model.DailyMetric{
					ID:             502,
					UserID:         12,
					Date:           time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
					ReadinessScore: 92,
				}
				m.On("GetDailyMetricByID", mock.Anything, int64(502), int64(12)).
					Return(metric, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(502), res.ID)
				assert.Equal(t, 92, res.ReadinessScore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			url := "/api/daily-metrics/" + tt.paramID
			if tt.queryURL != "" {
				url = tt.queryURL
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)

			if tt.paramID != "" {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", tt.paramID)
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			}

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetDailyMetricByID(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDailyMetricHandler_UpdateDailyMetric(t *testing.T) {
	validBody := `{"id": 501, "sleep_score": 88, "sleep_hours": 7.8, "body_battery_avg": 75, "steps": 11000}`

	tests := []struct {
		name           string
		authUserID     *int64
		body           string
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			body:           validBody,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON body",
			authUserID:     Ptr(int64(12)),
			body:           `{invalid_json}`,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:       "404 Not Found on service error",
			authUserID: Ptr(int64(12)),
			body:       validBody,
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("UpdateDailyMetric", mock.Anything, mock.MatchedBy(func(req dto.UpdateDailyMetricRequest) bool {
					return req.ID == 501 && req.UserID == 12
				})).Return(nil, apperrors.ErrDailyMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "daily metric not found")
			},
		},
		{
			name:       "200 Success",
			authUserID: Ptr(int64(12)),
			body:       validBody,
			mockBehavior: func(m *mocks.DailyMetricService) {
				updated := &model.DailyMetric{
					ID:             501,
					UserID:         12,
					Date:           time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
					SleepScore:     88,
					SleepHours:     7.8,
					BodyBatteryAvg: 75.0,
					Steps:          11000,
				}
				m.On("UpdateDailyMetric", mock.Anything, mock.MatchedBy(func(req dto.UpdateDailyMetricRequest) bool {
					return req.ID == 501 && req.UserID == 12
				})).Return(updated, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.DailyMetricResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(501), res.ID)
				assert.Equal(t, 88, res.SleepScore)
				assert.Equal(t, 7.8, res.SleepHours)
				assert.Equal(t, 75.0, res.BodyBatteryAvg)
				assert.Equal(t, 11000, res.Steps)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPut, "/api/daily-metrics", strings.NewReader(tt.body))
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.UpdateDailyMetric(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDailyMetricHandler_DeleteDailyMetric(t *testing.T) {
	tests := []struct {
		name           string
		paramID        string
		queryURL       string
		authUserID     *int64
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "400 Bad Request on invalid ID",
			paramID:        "not_an_int",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:           "401 Unauthorized",
			paramID:        "501",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:       "404 Not Found on service error",
			paramID:    "999",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("DeleteDailyMetric", mock.Anything, int64(999), int64(12)).
					Return(apperrors.ErrDailyMetricNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "daily metric not found")
			},
		},
		{
			name:       "204 Success via URLParam",
			paramID:    "501",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("DeleteDailyMetric", mock.Anything, int64(501), int64(12)).
					Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Empty(t, rec.Body.String())
			},
		},
		{
			name:       "204 Success via QueryParam fallback",
			paramID:    "", // no URLParam, fallback to ?id=502
			queryURL:   "/api/daily-metrics?id=502",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.DailyMetricService) {
				m.On("DeleteDailyMetric", mock.Anything, int64(502), int64(12)).
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
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			url := "/api/daily-metrics/" + tt.paramID
			if tt.queryURL != "" {
				url = tt.queryURL
			}
			req := httptest.NewRequest(http.MethodDelete, url, nil)

			if tt.paramID != "" {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", tt.paramID)
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			}

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.DeleteDailyMetric(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDailyMetricHandler_GenerateAdvice(t *testing.T) {
	validBody := `{"date": "2026-02-15"}`

	tests := []struct {
		name           string
		authUserID     *int64
		body           string
		mockBehavior   func(m *mocks.DailyMetricService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			body:           validBody,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON body",
			authUserID:     Ptr(int64(12)),
			body:           `{invalid_json}`,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:           "400 Bad Request on invalid date format",
			authUserID:     Ptr(int64(12)),
			body:           `{"date": "not_a_valid_date"}`,
			mockBehavior:   func(m *mocks.DailyMetricService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid date format, expected YYYY-MM-DD")
			},
		},
		{
			name:       "500 Internal Server Error on service failure",
			authUserID: Ptr(int64(12)),
			body:       validBody,
			mockBehavior: func(m *mocks.DailyMetricService) {
				expectedDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
				m.On("TriggerAdviceGeneration", mock.Anything, int64(12), expectedDate).
					Return(apperrors.ErrInternal).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "internal_error")
			},
		},
		{
			name:       "202 Accepted Success",
			authUserID: Ptr(int64(12)),
			body:       validBody,
			mockBehavior: func(m *mocks.DailyMetricService) {
				expectedDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
				m.On("TriggerAdviceGeneration", mock.Anything, int64(12), expectedDate).
					Return(nil).Once()
			},
			expectedStatus: http.StatusAccepted,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Empty(t, rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.DailyMetricService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewDailyMetricHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/daily-metrics/advice", strings.NewReader(tt.body))
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GenerateAdvice(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestToDailyMetricResponse_Nil(t *testing.T) {
	res := my_http.ExportToDailyMetricResponse(nil)
	assert.Equal(t, int64(0), res.ID)
}
