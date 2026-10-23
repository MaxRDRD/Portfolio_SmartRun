package service_test

import (
	"SmartRun/internal/config"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/apperrors"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPasskeyRepo
type MockPasskeyRepo struct {
	mock.Mock
}

func (m *MockPasskeyRepo) Create(ctx context.Context, passkey *model.Passkey) error {
	args := m.Called(ctx, passkey)
	return args.Error(0)
}

func (m *MockPasskeyRepo) GetByID(ctx context.Context, id int64) (*model.Passkey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Passkey), args.Error(1)
}

func (m *MockPasskeyRepo) GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error) {
	args := m.Called(ctx, credentialID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Passkey), args.Error(1)
}

func (m *MockPasskeyRepo) GetByUserID(ctx context.Context, userID int64) ([]model.Passkey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Passkey), args.Error(1)
}

func (m *MockPasskeyRepo) UpdateSignCountAndLastUsed(ctx context.Context, id int64, signCount uint32) error {
	args := m.Called(ctx, id, signCount)
	return args.Error(0)
}

func (m *MockPasskeyRepo) Delete(ctx context.Context, id int64, userID int64) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

// MockCache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Del(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := m.Called(ctx, keys)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCache) SetWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	args := m.Called(ctx, key, value, baseTTL)
	return args.Error(0)
}

func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func setupPasskeyTest(t *testing.T) (
	service.PasskeyService,
	*MockUserRepo,
	*MockPasskeyRepo,
	*MockSessionRepo,
	*MockTotpRepo,
	*MockTxManager,
	*MockCache,
) {
	t.Setenv("JWT_SECRET", "test-secret-key-32-bytes-long-1234")

	userRepo := new(MockUserRepo)
	passkeyRepo := new(MockPasskeyRepo)
	sessionRepo := new(MockSessionRepo)
	totpRepo := new(MockTotpRepo)
	txManager := new(MockTxManager)
	cacheMock := new(MockCache)

	w, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "SmartRun Test",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
	})
	require.NoError(t, err)

	cfg := config.AuthConfig{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	passkeySvc := service.NewPasskeyService(
		userRepo,
		passkeyRepo,
		sessionRepo,
		totpRepo,
		txManager,
		cacheMock,
		w,
		cfg,
	)

	return passkeySvc, userRepo, passkeyRepo, sessionRepo, totpRepo, txManager, cacheMock
}

func TestPasskeyService_BeginRegistration(t *testing.T) {
	ctx := context.Background()

	t.Run("User not found", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _ := setupPasskeyTest(t)

		userRepo.On("GetUserByID", ctx, int64(10)).Return(nil, apperrors.ErrUserNotFound)

		creation, err := svc.BeginRegistration(ctx, 10)
		require.Error(t, err)
		assert.Nil(t, creation)
		userRepo.AssertExpectations(t)
	})

	t.Run("Success", func(t *testing.T) {
		svc, userRepo, passkeyRepo, _, _, _, cacheMock := setupPasskeyTest(t)

		user := &model.User{
			ID:    10,
			Email: "runner@smartrun.app",
			Name:  "Test Runner",
		}
		userRepo.On("GetUserByID", ctx, int64(10)).Return(user, nil)
		passkeyRepo.On("GetByUserID", ctx, int64(10)).Return([]model.Passkey{}, nil)
		cacheMock.On("Set", ctx, "passkey:reg:10", mock.AnythingOfType("string"), 5*time.Minute).Return(nil)

		creation, err := svc.BeginRegistration(ctx, 10)
		require.NoError(t, err)
		assert.NotNil(t, creation)
		assert.NotEmpty(t, creation.Response.Challenge)

		userRepo.AssertExpectations(t)
		passkeyRepo.AssertExpectations(t)
		cacheMock.AssertExpectations(t)
	})
}

func TestPasskeyService_FinishRegistration(t *testing.T) {
	ctx := context.Background()

	t.Run("Session expired", func(t *testing.T) {
		svc, _, _, _, _, _, cacheMock := setupPasskeyTest(t)

		cacheMock.On("Get", ctx, "passkey:reg:10").Return("", errors.New("redis nil"))

		pk, err := svc.FinishRegistration(ctx, 10, strings.NewReader("{}"), "MacBook")
		require.Error(t, err)
		assert.Nil(t, pk)
		assert.True(t, errors.Is(err, apperrors.ErrPasskeySessionExpired))
	})
}

func TestPasskeyService_BeginLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("With email - user not found", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _ := setupPasskeyTest(t)

		userRepo.On("GetUserByEmail", ctx, "ghost@smartrun.app").Return(nil, apperrors.ErrUserNotFound)

		assertion, sessionID, err := svc.BeginLogin(ctx, "ghost@smartrun.app")
		require.Error(t, err)
		assert.Empty(t, sessionID)
		assert.Nil(t, assertion)
		assert.True(t, errors.Is(err, apperrors.ErrUserNotFound))
	})

	t.Run("With email - user has no passkeys", func(t *testing.T) {
		svc, userRepo, passkeyRepo, _, _, _, _ := setupPasskeyTest(t)

		user := &model.User{ID: 10, Email: "nopasskeys@smartrun.app"}
		userRepo.On("GetUserByEmail", ctx, "nopasskeys@smartrun.app").Return(user, nil)
		passkeyRepo.On("GetByUserID", ctx, int64(10)).Return([]model.Passkey{}, nil)

		assertion, sessionID, err := svc.BeginLogin(ctx, "nopasskeys@smartrun.app")
		require.Error(t, err)
		assert.Empty(t, sessionID)
		assert.Nil(t, assertion)
		assert.True(t, errors.Is(err, apperrors.ErrPasskeyNotFound))
	})

	t.Run("Discoverable login - success", func(t *testing.T) {
		svc, _, _, _, _, _, cacheMock := setupPasskeyTest(t)

		cacheMock.On("Set", ctx, mock.MatchedBy(func(k string) bool {
			return strings.HasPrefix(k, "passkey:login:")
		}), mock.AnythingOfType("string"), 5*time.Minute).Return(nil)

		assertion, sessionID, err := svc.BeginLogin(ctx, "")
		require.NoError(t, err)
		assert.NotEmpty(t, sessionID)
		assert.NotNil(t, assertion)
		assert.NotEmpty(t, assertion.Response.Challenge)
	})
}

func TestPasskeyService_FinishLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("Session expired or missing", func(t *testing.T) {
		svc, _, _, _, _, _, cacheMock := setupPasskeyTest(t)

		cacheMock.On("Get", ctx, "passkey:login:bad-session").Return("", errors.New("not found"))

		res, err := svc.FinishLogin(ctx, "bad-session", strings.NewReader("{}"))
		require.Error(t, err)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, apperrors.ErrPasskeySessionExpired))
	})
}

func TestPasskeyService_GetUserPasskeys(t *testing.T) {
	ctx := context.Background()
	svc, _, passkeyRepo, _, _, _, _ := setupPasskeyTest(t)

	expected := []model.Passkey{
		{ID: 1, UserID: 10, DeviceName: "iPhone 15"},
		{ID: 2, UserID: 10, DeviceName: "YubiKey 5"},
	}

	passkeyRepo.On("GetByUserID", ctx, int64(10)).Return(expected, nil)

	result, err := svc.GetUserPasskeys(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	passkeyRepo.AssertExpectations(t)
}

func TestPasskeyService_DeletePasskey(t *testing.T) {
	ctx := context.Background()
	svc, _, passkeyRepo, _, _, _, _ := setupPasskeyTest(t)

	passkeyRepo.On("Delete", ctx, int64(1), int64(10)).Return(nil)

	err := svc.DeletePasskey(ctx, 1, 10)
	require.NoError(t, err)
	passkeyRepo.AssertExpectations(t)
}
