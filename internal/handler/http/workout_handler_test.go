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
	"errors"
	"io"
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

type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestWorkoutHandler_Create(t *testing.T) {
	validBody := `{"distance": 10.0, "duration": 3600, "type_activity": "running"}`

	tests := []struct {
		name           string
		authUserID     *int64
		contentType    string
		body           string
		bodyReader     io.Reader
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			authUserID:     nil,
			contentType:    "application/json",
			body:           `{invalid}`,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Bad Request on invalid JSON syntax",
			authUserID:     Ptr(int64(12)),
			contentType:    "application/json",
			body:           `{invalid_body}`,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid JSON body")
			},
		},
		{
			name:        "400 Bad Request on service",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("Create", mock.Anything, int64(12), mock.AnythingOfType("dto.CreateRequest")).
					Return(nil, my_errors.ErrInvalidData).Once()
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid data")
			},
		},
		{
			name:        "400 Bad Request on .fit parse (empty body)",
			authUserID:  Ptr(int64(12)),
			contentType: "",
			body:        "",
			mockBehavior: func(m *mocks.WorkoutService) {

			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "empty file")
			},
		},
		{
			name:        "400 Bad Request on failed to read fit body",
			authUserID:  Ptr(int64(12)),
			contentType: "application/octet-stream",
			bodyReader:  errReader{},
			mockBehavior: func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to read file")
			},
		},
		{
			name:        "201 Created on .fit file",
			authUserID:  Ptr(int64(12)),
			contentType: "application/octet-stream",
			body:        "fake-fit-binary-content",
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := &model.Workouts{
					ID:           42,
					Distance:     10.5,
					Duration:     3600,
					TypeActivity: "running",
				}
				m.On("UploadFit", mock.Anything, int64(12), []byte("fake-fit-binary-content")).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(42), res.ID)
				assert.Equal(t, 10.5, res.Distance)
				assert.Equal(t, "running", res.TypeActivity)
			},
		},
		{
			name:        "400 Bad Request on corrupted .fit file",
			authUserID:  Ptr(int64(12)),
			contentType: "application/octet-stream",
			body:        "corrupted-fit-data",
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("UploadFit", mock.Anything, int64(12), []byte("corrupted-fit-data")).
					Return(nil, my_errors.ErrInvalidData).Once()
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid data")
			},
		},
		{
			name:           "415 unsupported media type",
			authUserID:     Ptr(int64(12)),
			contentType:    "wrong_content_type",
			body:           validBody,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnsupportedMediaType,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unsupported Content-Type: wrong_content_type")
			},
		},
		{
			name:        "500 Internal Error when service returns nil workout without error",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("Create", mock.Anything, int64(12), mock.AnythingOfType("dto.CreateRequest")).
					Return(nil, nil).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "internal_error")
			},
		},
		{
			name:        "201 Success",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := &model.Workouts{
					ID:           int64(12),
					Distance:     10.0,
					Duration:     3600,
					TypeActivity: "running",
				}
				m.On("Create", mock.Anything, int64(12), mock.AnythingOfType("dto.CreateRequest")).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, 10.0, res.Distance)
				assert.Equal(t, 3600, res.Duration)
				assert.Equal(t, "running", res.TypeActivity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			var reader io.Reader
			if tt.bodyReader != nil {
				reader = tt.bodyReader
			} else {
				reader = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/workout/create", reader)

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			h.Create(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestWorkoutHandler_Update(t *testing.T) {
	validBody := `{"distance": 10.0, "duration": 3600, "type_activity": "running"}`

	tests := []struct {
		name           string
		method         string
		paramID        string
		authUserID     *int64
		contentType    string
		body           string
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "400 Invalid body",
			method:         http.MethodPatch,
			paramID:        "1",
			authUserID:     Ptr(int64(12)),
			contentType:    "application/json",
			body:           "invalid_body",
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:           "401 Unauthorized",
			method:         http.MethodPatch,
			paramID:        "1",
			authUserID:     nil,
			contentType:    "application/json",
			body:           validBody,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Invalid id",
			method:         http.MethodPatch,
			paramID:        "abc",
			authUserID:     Ptr(int64(12)),
			contentType:    "application/json",
			body:           validBody,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:        "404 Not Found on service",
			method:      http.MethodPatch,
			paramID:     "999",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("Update", mock.Anything, int64(12), int64(999), mock.AnythingOfType("dto.UpdateRequest"), false).
					Return(nil, my_errors.ErrWorkoutNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "workout not found")
			},
		},
		{
			name:        "200 Success PATCH",
			method:      http.MethodPatch,
			paramID:     "1",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := &model.Workouts{
					ID:           int64(1),
					Distance:     10.0,
					Duration:     3600,
					TypeActivity: "running",
				}
				m.On("Update", mock.Anything, int64(12), int64(1), mock.AnythingOfType("dto.UpdateRequest"), false).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(1), res.ID)
				assert.Equal(t, 10.0, res.Distance)
				assert.Equal(t, 3600, res.Duration)
				assert.Equal(t, "running", res.TypeActivity)
			},
		},
		{
			name:        "200 Success PUT (Full Replace)",
			method:      http.MethodPut,
			paramID:     "1",
			authUserID:  Ptr(int64(12)),
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := &model.Workouts{
					ID:           int64(1),
					Distance:     10.0,
					Duration:     3600,
					TypeActivity: "running",
				}
				m.On("Update", mock.Anything, int64(12), int64(1), mock.AnythingOfType("dto.UpdateRequest"), true).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(1), res.ID)
				assert.Equal(t, 10.0, res.Distance)
				assert.Equal(t, 3600, res.Duration)
				assert.Equal(t, "running", res.TypeActivity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			method := tt.method
			if method == "" {
				method = http.MethodPatch
			}
			req := httptest.NewRequest(method, "/api/workout/"+tt.paramID, strings.NewReader(tt.body))

			// Добавляем chi URL param "id"
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			h.Update(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestWorkoutHandler_GetAll(t *testing.T) {
	tests := []struct {
		name           string
		queryURL       string
		authUserID     *int64
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			queryURL:       "/api/workout/",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Invalid query params",
			queryURL:       "/api/workout/?min_distance=not_a_number",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid query param")
			},
		},
		{
			name:           "400 Invalid from date format",
			queryURL:       "/api/workout/?from=not_a_date",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "expected format YYYY-MM-DD")
			},
		},
		{
			name:           "400 Invalid max_distance",
			queryURL:       "/api/workout/?max_distance=not_a_float",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "must be a number")
			},
		},
		{
			name:           "400 Invalid min_duration",
			queryURL:       "/api/workout/?min_duration=not_an_int",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "must be an integer")
			},
		},
		{
			name:           "400 Invalid has_notes",
			queryURL:       "/api/workout/?has_notes=not_a_bool",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "must be true/false")
			},
		},
		{
			name:           "400 Invalid limit (<= 0)",
			queryURL:       "/api/workout/?limit=0",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid query param 'limit'")
			},
		},
		{
			name:           "400 Invalid offset (< 0)",
			queryURL:       "/api/workout/?offset=-1",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid query param 'offset'")
			},
		},
		{
			name:       "404 Not Found",
			queryURL:   "/api/workout/",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("GetAllByID", mock.Anything, mock.AnythingOfType("dto.WorkoutFilter")).
					Return(([]model.Workouts)(nil), my_errors.ErrWorkoutNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "workout not found")
			},
		},
		{
			name:       "200 Success with full query filters",
			queryURL:   "/api/workout/?type=running&shoes=Nike&limit=10&offset=5&min_distance=5.0&from=2026-01-01&to=2026-01-31&has_notes=true&has_hr_zones=false",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("GetAllByID", mock.Anything, mock.MatchedBy(func(f dto.WorkoutFilter) bool {
					return f.UserID == 12 && f.Type == "running" && f.Shoes == "Nike" && f.Limit == 10 && f.Offset == 5 &&
						f.MinDistance != nil && *f.MinDistance == 5.0 &&
						f.HasNotes != nil && *f.HasNotes == true &&
						f.HasHRZones != nil && *f.HasHRZones == false
				})).Return([]model.Workouts{}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res []dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Empty(t, res)
			},
		},
		{
			name:       "200 Success",
			queryURL:   "/api/workout/",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := []model.Workouts{
					{
						ID:           int64(1),
						Distance:     10.0,
						Duration:     3600,
						TypeActivity: "running",
					},
					{
						ID:           int64(2),
						Distance:     12.0,
						Duration:     3600,
						TypeActivity: "running",
					},
				}
				m.On("GetAllByID", mock.Anything, mock.AnythingOfType("dto.WorkoutFilter")).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res []dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				require.Len(t, res, 2)
				assert.Equal(t, int64(1), res[0].ID)
				assert.Equal(t, 10.0, res[0].Distance)
				assert.Equal(t, 3600, res[0].Duration)
				assert.Equal(t, "running", res[0].TypeActivity)
				assert.Equal(t, int64(2), res[1].ID)
				assert.Equal(t, 12.0, res[1].Distance)
				assert.Equal(t, 3600, res[1].Duration)
				assert.Equal(t, "running", res[1].TypeActivity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, tt.queryURL, nil)

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetAll(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestWorkoutHandler_GetHistoryByMonth(t *testing.T) {
	tests := []struct {
		name           string
		queryURL       string
		authUserID     *int64
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			queryURL:       "/api/workout/history",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:       "404 Not Found on service",
			queryURL:   "/api/workout/history",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("GetMonthlyHistory", mock.Anything, int64(12), 3, 0).
					Return(([]model.WorkoutMonthHistory)(nil), my_errors.ErrWorkoutNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "workout not found")
			},
		},
		{
			name:       "200 Success with default pagination (limit=3, offset=0)",
			queryURL:   "/api/workout/history",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				month1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
				month2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				expected := []model.WorkoutMonthHistory{
					{
						Month:         month1,
						WorkoutsCount: 10,
						TotalDistance: 100.0,
						TotalDuration: 10800,
						Workouts: []model.WorkoutPreview{
							{
								ID:           1,
								Date:         month1,
								Distance:     10.0,
								Duration:     3600,
								TypeActivity: "running",
							},
						},
					},
					{
						Month:         month2,
						WorkoutsCount: 12,
						TotalDistance: 120.0,
						TotalDuration: 12800,
					},
				}
				m.On("GetMonthlyHistory", mock.Anything, int64(12), 3, 0).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutHistoryResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				require.Len(t, res.Months, 2)
				assert.Equal(t, "2026-02", res.Months[0].Month)
				assert.Equal(t, 10, res.Months[0].WorkoutsCount)
				assert.Equal(t, 100.0, res.Months[0].TotalDistance)
				assert.Equal(t, 10800, res.Months[0].TotalDuration)
				require.Len(t, res.Months[0].Workouts, 1)
				assert.Equal(t, int64(1), res.Months[0].Workouts[0].ID)
				assert.Equal(t, "2026-01", res.Months[1].Month)
			},
		},
		{
			name:       "200 Success with custom query params (?limit=6&offset=2)",
			queryURL:   "/api/workout/history?limit=6&offset=2",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("GetMonthlyHistory", mock.Anything, int64(12), 6, 2).
					Return([]model.WorkoutMonthHistory{}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutHistoryResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Empty(t, res.Months)
			},
		},
		{
			name:       "200 Success with clamped query params (?limit=50&offset=-5)",
			queryURL:   "/api/workout/history?limit=50&offset=-5",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				// limit > 12 clamps to 12, offset < 0 clamps to 0
				m.On("GetMonthlyHistory", mock.Anything, int64(12), 12, 0).
					Return([]model.WorkoutMonthHistory{}, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutHistoryResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Empty(t, res.Months)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, tt.queryURL, nil)

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetHistoryByMonth(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestWorkoutHandler_GetByID(t *testing.T) {
	tests := []struct {
		name           string
		queryURL       string
		paramID        string
		authUserID     *int64
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			queryURL:       "/api/workout/",
			authUserID:     nil,
			paramID:        "1",
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Invalid query params",
			queryURL:       "/api/workout/",
			paramID:        "abc",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:       "404 Not Found",
			queryURL:   "/api/workout/",
			paramID:    "1",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("GetByID", mock.Anything, int64(1), int64(12)).
					Return(nil, my_errors.ErrWorkoutNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "workout not found")
			},
		},
		{
			name:       "200 Success",
			queryURL:   "/api/workout/",
			authUserID: Ptr(int64(12)),
			paramID:    "1",
			mockBehavior: func(m *mocks.WorkoutService) {
				expected := &model.Workouts{
					ID:           int64(1),
					Distance:     10.0,
					Duration:     3600,
					TypeActivity: "running",
				}
				m.On("GetByID", mock.Anything, int64(1), int64(12)).
					Return(expected, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.WorkoutsResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(1), res.ID)
				assert.Equal(t, 10.0, res.Distance)
				assert.Equal(t, 3600, res.Duration)
				assert.Equal(t, "running", res.TypeActivity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			req := httptest.NewRequest(http.MethodGet, tt.queryURL+tt.paramID, nil)

			// Добавляем chi URL param "id"
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.GetByID(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestWorkoutHandler_Delete(t *testing.T) {
	tests := []struct {
		name           string
		queryURL       string
		paramID        string
		authUserID     *int64
		mockBehavior   func(m *mocks.WorkoutService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "401 Unauthorized",
			queryURL:       "/api/workout/",
			authUserID:     nil,
			paramID:        "1",
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
		{
			name:           "400 Invalid query params",
			queryURL:       "/api/workout/",
			paramID:        "abc",
			authUserID:     Ptr(int64(12)),
			mockBehavior:   func(m *mocks.WorkoutService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid id")
			},
		},
		{
			name:       "404 Not Found",
			queryURL:   "/api/workout/",
			paramID:    "1",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.WorkoutService) {
				m.On("Delete", mock.Anything, int64(1), int64(12)).
					Return(my_errors.ErrWorkoutNotFound).Once()
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "workout not found")
			},
		},
		{
			name:       "204 Success",
			queryURL:   "/api/workout/",
			authUserID: Ptr(int64(12)),
			paramID:    "1",
			mockBehavior: func(m *mocks.WorkoutService) {

				m.On("Delete", mock.Anything, int64(1), int64(12)).
					Return(nil).Once()
			},
			expectedStatus: http.StatusNoContent,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {

			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.WorkoutService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewWorkoutHandler(mockSvc)

			req := httptest.NewRequest(http.MethodDelete, tt.queryURL+tt.paramID, nil)

			// Добавляем chi URL param "id"
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}

			rec := httptest.NewRecorder()
			h.Delete(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}
