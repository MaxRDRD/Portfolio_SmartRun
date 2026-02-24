package user_test

import (
	"SmartRun/internal/user"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

/*
	Register(ctx context.Context, req RegisterRequest) (*User, error)
	Login(ctx context.Context, req LoginRequest) (string, error)
	GetByID(ctx context.Context, id int) (*User, error)
*/

type MockService struct {
	mock.Mock
}

func (m *MockService) Register(
	ctx context.Context,
	req user.RegisterRequest,
) (*user.User, error) {

	args := m.Called(ctx, req)

	if u := args.Get(0); u != nil {
		return u.(*user.User), args.Error(1)
	}

	return nil, args.Error(1)
}

func (m *MockService) GetByID(ctx context.Context, id int) (*user.User, error) {
	args := m.Called(ctx, id)
	if myUser := args.Get(0); myUser != nil {
		return myUser.(*user.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockService) Login(ctx context.Context, req user.LoginRequest) (string, error) {
	args := m.Called(ctx, req)
	return args.String(0), args.Error(1)
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name              string
		body              string
		setup             func(m *MockService)
		wantStatus        int
		expectServiceCall bool
	}{
		{
			name: "success",
			body: `{"email":"email@example.com","password":"password123"}`,
			setup: func(m *MockService) {
				m.On("Register", mock.Anything, user.RegisterRequest{
					Email:    "email@example.com",
					Password: "password123",
				}).Return(&user.User{ID: 1}, nil).Once()
			},
			wantStatus:        http.StatusCreated,
			expectServiceCall: true,
		},
		{
			name:              "invalid json",
			body:              `{"email":"test@example.com",`,
			setup:             func(m *MockService) {},
			wantStatus:        http.StatusBadRequest,
			expectServiceCall: false,
		},
		{
			name: "user already exists",
			body: `{"email":"email@example.com","password":"password123"}`,
			setup: func(m *MockService) {
				m.On("Register", mock.Anything, mock.Anything).
					Return(nil, user.ErrUserAlreadyExists).
					Once()
			},
			wantStatus:        http.StatusConflict,
			expectServiceCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockService := new(MockService)
			handler := user.NewHandler(mockService)

			tt.setup(mockService)

			req := httptest.NewRequest(
				http.MethodPost,
				"/register",
				strings.NewReader(tt.body),
			)

			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			handler.Register(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if !tt.expectServiceCall {
				mockService.AssertNotCalled(t, "Register", mock.Anything, mock.Anything)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name              string
		body              string
		setup             func(m *MockService)
		wantStatus        int
		expectServiceCall bool
	}{
		{
			name: "success",
			body: `{"email":"email@example.com","password":"password123"}`,
			setup: func(m *MockService) {
				m.On("Login", mock.Anything, user.LoginRequest{
					Email:    "email@example.com",
					Password: "password123",
				}).Return("mocked-jwt-token", nil).Once()
			},
			wantStatus:        http.StatusOK,
			expectServiceCall: true,
		},
		{
			name:              "invalid json",
			body:              `{"email":"email@example.com",`,
			setup:             func(m *MockService) {},
			wantStatus:        http.StatusBadRequest,
			expectServiceCall: false,
		},
		{
			name: "invalid credentials",
			body: `{"email":"email@example.com","password":"wrong"}`,
			setup: func(m *MockService) {
				m.On("Login", mock.Anything, mock.Anything).
					Return("", errors.New("invalid credentials")).
					Once()
			},
			wantStatus:        http.StatusUnauthorized,
			expectServiceCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockService := new(MockService)
			handler := user.NewHandler(mockService)

			tt.setup(mockService)

			req := httptest.NewRequest(
				http.MethodPost,
				"/login",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			handler.Login(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusBadRequest {
				mockService.AssertNotCalled(t, "Login", mock.Anything, mock.Anything)
			}

			mockService.AssertExpectations(t)
		})
	}
}
