package http_test

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	my_http "SmartRun/internal/handler/http"
	"SmartRun/internal/handler/http/mocks"
	"SmartRun/pkg/my_errors"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_Register(t *testing.T) {
	validBody := `{"name": "Maxim", "email": "max@example.com", "password": "strongpassword123"}`

	tests := []struct {
		name           string
		contentType    string
		body           string
		mockBehavior   func(m *mocks.AuthService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:        "Success 201 Created",
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.AuthService) {
				expectedResult := &dto.AuthResult{
					AccessToken:  "mock-access-token",
					RefreshToken: "mock-refresh-token",
					ExpiresIn:    900,
					User: &dto.UserResponse{
						ID:    12,
						Name:  "Maxim",
						Email: "max@example.com",
					},
				}
				m.On("Register", mock.Anything, mock.AnythingOfType("dto.RegisterRequest")).
					Return(expectedResult, nil).
					Once()
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// 1. Проверяем структуру ответа: { "user": {...}, "auth": {...} }
				var res struct {
					User dto.UserResponse `json:"user"`
					Auth dto.AuthResponse `json:"auth"`
				}
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, int64(12), res.User.ID)
				assert.Equal(t, "Maxim", res.User.Name)
				assert.Equal(t, "max@example.com", res.User.Email)
				assert.Equal(t, "mock-access-token", res.Auth.AccessToken)

				// 2. Проверяем установку refresh_token cookie
				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, "mock-refresh-token", cookies[0].Value)
				assert.True(t, cookies[0].HttpOnly)
				assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
			},
		},
		{
			name:        "400 Bad Request on invalid JSON body",
			contentType: "application/json",
			body:        `{broken-json`,
			mockBehavior: func(m *mocks.AuthService) {
				// Сервис не должен вызываться при битом JSON
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:        "409 Conflict when email already registered",
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.AuthService) {
				m.On("Register", mock.Anything, mock.AnythingOfType("dto.RegisterRequest")).
					Return(nil, my_errors.ErrUserAlreadyExists).
					Once()
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "user already exists")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AuthService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewUserHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			h.Register(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestUserHandler_Login(t *testing.T) {
	validBody := `{"email": "max@example.com", "password": "strongpassword123"}`

	tests := []struct {
		name           string
		contentType    string
		body           string
		mockBehavior   func(m *mocks.AuthService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:        "Success 200 Ok",
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.AuthService) {
				expectedResult := &dto.AuthResult{
					AccessToken:  "mock-access-token",
					RefreshToken: "mock-refresh-token",
					ExpiresIn:    900,
					User: &dto.UserResponse{
						ID:          12,
						Name:        "Maxim",
						Email:       "max@example.com",
						TOTPEnabled: false,
					},
				}
				m.On("Login", mock.Anything, mock.AnythingOfType("dto.LoginRequest")).
					Return(expectedResult, nil).
					Once()
				m.On("IsTOTPEnabled", mock.Anything, int64(12)).
					Return(false, nil).
					Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.AuthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, "mock-access-token", res.AccessToken)
				assert.Equal(t, 900, res.ExpiresIn)
				assert.False(t, res.Require2FA)
				assert.Equal(t, "ok", res.Message)

				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, "mock-refresh-token", cookies[0].Value)
				assert.True(t, cookies[0].HttpOnly)
				assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
			},
		},
		{
			name:        "Success with 2FA",
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.AuthService) {
				expected := &dto.AuthResult{
					AccessToken:  "",
					RefreshToken: "mock-refresh-token",
					ExpiresIn:    900,
					User: &dto.UserResponse{
						ID:          12,
						Name:        "Maxim",
						Email:       "max@example.com",
						TOTPEnabled: true,
					},
				}
				m.On("Login", mock.Anything, mock.AnythingOfType("dto.LoginRequest")).
					Return(expected, nil).
					Once()
				m.On("IsTOTPEnabled", mock.Anything, int64(12)).
					Return(true, nil).
					Once()
			},
			expectedStatus: http.StatusAccepted, // 202 Accepted!
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.AuthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.True(t, res.Require2FA)
				assert.Empty(t, res.AccessToken)
				assert.Contains(t, res.Message, "2FA required")

				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, "mock-refresh-token", cookies[0].Value)
				assert.True(t, cookies[0].HttpOnly)
			},
		},
		{
			name:           "400 Bad Request",
			contentType:    "application/json",
			body:           `{"email":`,
			mockBehavior:   func(m *mocks.AuthService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid request body")
			},
		},
		{
			name:        "401 Unauthorized",
			contentType: "application/json",
			body:        validBody,
			mockBehavior: func(m *mocks.AuthService) {
				m.On("Login", mock.Anything, mock.AnythingOfType("dto.LoginRequest")).
					Return(nil, my_errors.ErrInvalidCredentials).
					Once()
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid credentials")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AuthService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewUserHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			h.Login(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestUserHandler_Refresh(t *testing.T) {
	tests := []struct {
		name           string
		cookie         *http.Cookie // входная кука (nil если клиент не прислал)
		mockBehavior   func(m *mocks.AuthService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "Success 200 OK: rotates refresh token and returns new access token",
			cookie: &http.Cookie{
				Name:  "refresh_token",
				Value: "valid-old-refresh-token",
			},
			mockBehavior: func(m *mocks.AuthService) {
				expectedResult := &dto.AuthResult{
					AccessToken:  "new-access-token-123",
					RefreshToken: "new-refresh-token-456",
					ExpiresIn:    900,
				}
				m.On("Refresh", mock.Anything, "valid-old-refresh-token").
					Return(expectedResult, nil).
					Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// 1. Проверяем тело ответа (новый access_token)
				var res dto.AuthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				assert.Equal(t, "new-access-token-123", res.AccessToken)
				assert.Equal(t, 900, res.ExpiresIn)
				assert.Equal(t, "ok", res.Message)

				// 2. Проверяем новую куку в ответе
				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, "new-refresh-token-456", cookies[0].Value)
				assert.True(t, cookies[0].HttpOnly)
				assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
			},
		},
		{
			name:   "401 Unauthorized: missing refresh token cookie",
			cookie: nil, // клиент не прислал куку
			mockBehavior: func(m *mocks.AuthService) {
				// Сервис не должен вызываться
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "refresh token not found")
			},
		},
		{
			name: "401 Unauthorized: token expired or invalid in database",
			cookie: &http.Cookie{
				Name:  "refresh_token",
				Value: "expired-or-revoked-token",
			},
			mockBehavior: func(m *mocks.AuthService) {
				m.On("Refresh", mock.Anything, "expired-or-revoked-token").
					Return(nil, my_errors.ErrTokenNotFound).
					Once()
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "invalid refresh token")

				// При ошибке хендлер инвалидирует куку (MaxAge: -1)
				cookies := rec.Result().Cookies()
				require.NotEmpty(t, cookies)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, -1, cookies[0].MaxAge)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AuthService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewUserHandler(mockSvc)

			// В Refresh теле запроса нет - передаем nil
			req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}

			rec := httptest.NewRecorder()
			h.Refresh(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestUserHandler_Logout(t *testing.T) {
	tests := []struct {
		name           string
		cookie         *http.Cookie
		mockBehavior   func(m *mocks.AuthService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "Success: deletes session and clears cookie",
			cookie: &http.Cookie{
				Name:  "refresh_token",
				Value: "active-token",
			},
			mockBehavior: func(m *mocks.AuthService) {
				m.On("Logout", mock.Anything, "active-token").
					Return(nil).
					Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "logged out")

				// Кука должна затираться (MaxAge: -1)
				cookies := rec.Result().Cookies()
				require.NotEmpty(t, cookies)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, -1, cookies[0].MaxAge)
			},
		},
		{
			name:   "Success even if cookie was missing",
			cookie: nil,
			mockBehavior: func(m *mocks.AuthService) {
				// Logout в сервисе не вызывается, если куки нет
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "logged out")

				cookies := rec.Result().Cookies()
				require.NotEmpty(t, cookies)
				assert.Equal(t, "refresh_token", cookies[0].Name)
				assert.Equal(t, -1, cookies[0].MaxAge)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AuthService)
			tt.mockBehavior(mockSvc)

			h := my_http.NewUserHandler(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}

			rec := httptest.NewRecorder()
			h.Logout(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestUserHandler_Me(t *testing.T) {
	tests := []struct {
		name           string
		authUserID     *int64
		mockBehavior   func(m *mocks.AuthService)
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:       "Success 200 OK: returns current user profile",
			authUserID: Ptr(int64(12)),
			mockBehavior: func(m *mocks.AuthService) {
				m.On("GetUserByID", mock.Anything, int64(12)).
					Return(&dto.UserResponse{ID: 12, Name: "Maxim", Email: "max@example.com"}, nil).Once()
				m.On("IsTOTPEnabled", mock.Anything, int64(12)).
					Return(false, nil).Once()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var res dto.UserResponse
				err := json.Unmarshal(rec.Body.Bytes(), &res)

				require.NoError(t, err)
				require.Equal(t, res.ID, int64(12))
				require.Equal(t, res.TOTPEnabled, false)
				require.Equal(t, "Maxim", res.Name)
			},
		},
		{
			name:           "401 Unauthorized: missing user in context",
			authUserID:     nil,
			mockBehavior:   func(m *mocks.AuthService) {},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AuthService)
			tt.mockBehavior(mockSvc)
			h := my_http.NewUserHandler(mockSvc)
			req := httptest.NewRequest(http.MethodGet, "/api/user/me", nil)
			if tt.authUserID != nil {
				req = req.WithContext(auth.SetUserID(req.Context(), *tt.authUserID))
			}
			rec := httptest.NewRecorder()
			h.Me(rec, req)
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func Ptr[T any](v T) *T {
	return &v
}
