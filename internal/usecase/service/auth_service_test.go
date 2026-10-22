package service_test

import (
	"SmartRun/internal/broker"
	"SmartRun/internal/config"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/apperrors"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthTest(t *testing.T) (
	service.AuthService,
	*MockUserRepo,
	*MockSessionRepo,
	*MockTotpRepo,
	*MockPasswordResetRepo,
	*MockEmailService,
	*MockTxManager,
	*MockPublisher,
) {
	t.Setenv("JWT_SECRET", "test-secret-key-32-bytes-long-1234")

	userRepo := new(MockUserRepo)
	sessionRepo := new(MockSessionRepo)
	totpRepo := new(MockTotpRepo)
	passwordResetRepo := new(MockPasswordResetRepo)
	emailService := new(MockEmailService)
	txManager := new(MockTxManager)
	publisher := new(MockPublisher)

	v := validator.New()
	cfg := config.AuthConfig{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		PublicURL:       "http://localhost:3000",
	}

	authSvc := service.NewUserService(
		userRepo,
		sessionRepo,
		totpRepo,
		passwordResetRepo,
		emailService,
		cfg,
		txManager,
		v,
		publisher,
	)

	return authSvc, userRepo, sessionRepo, totpRepo, passwordResetRepo, emailService, txManager, publisher
}

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()
	age := 28
	weight := 72.5
	height := 180.0

	t.Run("success", func(t *testing.T) {
		svc, userRepo, sessionRepo, _, _, _, txManager, _ := setupAuthTest(t)

		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		userRepo.On("GetUserByEmail", ctx, "runner@example.com").Return(nil, apperrors.ErrUserNotFound)
		userRepo.On("CreateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "runner@example.com" && u.Name == "Ivan Runner" && u.Age == 28
		})).Run(func(args mock.Arguments) {
			u := args.Get(1).(*model.User)
			u.ID = 42
		}).Return(nil)
		sessionRepo.On("CreateSession", ctx, mock.MatchedBy(func(s *model.Session) bool {
			return s.UserId == 42 && !s.Revoked && s.RefreshHash != ""
		})).Return(nil)

		req := dto.RegisterRequest{
			Name:     "Ivan Runner",
			Email:    " Runner@Example.com ",
			Password: "strong_password_123",
			Age:      &age,
			WeightKg: &weight,
			HeightCm: &height,
		}

		res, err := svc.Register(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, int64(42), res.User.ID)
		assert.Equal(t, "runner@example.com", res.User.Email)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.Equal(t, 900, res.ExpiresIn)

		txManager.AssertExpectations(t)
		userRepo.AssertExpectations(t)
		sessionRepo.AssertExpectations(t)
	})

	t.Run("validation failure - short password", func(t *testing.T) {
		svc, _, _, _, _, _, _, _ := setupAuthTest(t)

		req := dto.RegisterRequest{
			Name:     "Ivan Runner",
			Email:    "runner@example.com",
			Password: "123", // min=8
		}

		res, err := svc.Register(ctx, req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("duplicate email", func(t *testing.T) {
		svc, userRepo, _, _, _, _, txManager, _ := setupAuthTest(t)

		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		userRepo.On("GetUserByEmail", ctx, "existing@example.com").Return(&model.User{ID: 10, Email: "existing@example.com"}, nil)

		req := dto.RegisterRequest{
			Name:     "Ivan Runner",
			Email:    "existing@example.com",
			Password: "strong_password_123",
		}

		res, err := svc.Register(ctx, req)
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrUserAlreadyExists))
		assert.Nil(t, res)
	})
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &model.User{
		ID:       101,
		Email:    "runner@example.com",
		Password: string(hashedPassword),
		Name:     "Fast Runner",
	}

	t.Run("success without 2fa", func(t *testing.T) {
		svc, userRepo, sessionRepo, totpRepo, _, _, txManager, _ := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "runner@example.com").Return(user, nil)
		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		sessionRepo.On("DeleteAllSessionsForUser", ctx, int64(101)).Return(nil)
		sessionRepo.On("CreateSession", ctx, mock.MatchedBy(func(s *model.Session) bool {
			return s.UserId == 101 && s.RefreshHash != ""
		})).Return(nil)
		totpRepo.On("IsTOTPEnabled", ctx, int64(101)).Return(false, nil)

		req := dto.LoginRequest{
			Email:    "runner@example.com",
			Password: "correct_password",
		}

		res, err := svc.Login(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, int64(101), res.User.ID)
		assert.False(t, res.User.TOTPEnabled)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)

		userRepo.AssertExpectations(t)
		sessionRepo.AssertExpectations(t)
		totpRepo.AssertExpectations(t)
	})

	t.Run("success with 2fa enabled flag in response", func(t *testing.T) {
		svc, userRepo, sessionRepo, totpRepo, _, _, txManager, _ := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "runner@example.com").Return(user, nil)
		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		sessionRepo.On("DeleteAllSessionsForUser", ctx, int64(101)).Return(nil)
		sessionRepo.On("CreateSession", ctx, mock.Anything).Return(nil)
		totpRepo.On("IsTOTPEnabled", ctx, int64(101)).Return(true, nil)

		req := dto.LoginRequest{
			Email:    "runner@example.com",
			Password: "correct_password",
		}

		res, err := svc.Login(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.True(t, res.User.TOTPEnabled)
	})

	t.Run("invalid password", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "runner@example.com").Return(user, nil)

		req := dto.LoginRequest{
			Email:    "runner@example.com",
			Password: "wrong_password",
		}

		res, err := svc.Login(ctx, req)
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrInvalidCredentials))
		assert.Nil(t, res)
	})

	t.Run("user not found", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "unknown@example.com").Return(nil, apperrors.ErrUserNotFound)

		req := dto.LoginRequest{
			Email:    "unknown@example.com",
			Password: "correct_password",
		}

		res, err := svc.Login(ctx, req)
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrInvalidCredentials))
		assert.Nil(t, res)
	})

	t.Run("validation error", func(t *testing.T) {
		svc, _, _, _, _, _, _, _ := setupAuthTest(t)

		req := dto.LoginRequest{
			Email:    "invalid-email",
			Password: "123", // min=6
		}

		res, err := svc.Login(ctx, req)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestAuthService_GetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		userRepo.On("GetUserByID", ctx, int64(1)).Return(&model.User{
			ID:    1,
			Email: "test@example.com",
			Name:  "Alex",
		}, nil)

		res, err := svc.GetUserByID(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, int64(1), res.ID)
		assert.Equal(t, "Alex", res.Name)
	})

	t.Run("not found", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		userRepo.On("GetUserByID", ctx, int64(99)).Return(nil, apperrors.ErrUserNotFound)

		res, err := svc.GetUserByID(ctx, 99)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestAuthService_UpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("success updating user fields and email with cache invalidation", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		initialUser := &model.User{
			ID:    1,
			Email: "old@example.com",
			Name:  "Old Name",
		}
		userRepo.On("GetUserByID", ctx, int64(1)).Return(initialUser, nil)
		userRepo.On("GetUserByEmail", ctx, "new@example.com").Return(nil, apperrors.ErrUserNotFound)
		userRepo.On("UpdateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "new@example.com" && u.Name == "New Name"
		})).Return(nil)
		userRepo.On("InvalidateUserCache", ctx, int64(1), "old@example.com").Return()
		userRepo.On("InvalidateUserCache", ctx, int64(1), "new@example.com").Return()

		newName := "New Name"
		newEmail := "new@example.com"
		req := dto.UpdateUserRequest{
			Name:  &newName,
			Email: &newEmail,
		}

		res, err := svc.UpdateUser(ctx, 1, req)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "New Name", res.Name)
		assert.Equal(t, "new@example.com", res.Email)

		userRepo.AssertExpectations(t)
	})

	t.Run("email already taken by someone else", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		initialUser := &model.User{
			ID:    1,
			Email: "user1@example.com",
		}
		userRepo.On("GetUserByID", ctx, int64(1)).Return(initialUser, nil)
		userRepo.On("GetUserByEmail", ctx, "taken@example.com").Return(&model.User{
			ID:    2,
			Email: "taken@example.com",
		}, nil)

		takenEmail := "taken@example.com"
		req := dto.UpdateUserRequest{
			Email: &takenEmail,
		}

		res, err := svc.UpdateUser(ctx, 1, req)
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrUserAlreadyExists))
		assert.Nil(t, res)
	})
}

func TestAuthService_GetEmailByID(t *testing.T) {
	ctx := context.Background()
	svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

	userRepo.On("GetEmailByID", ctx, int64(5)).Return("user5@example.com", nil)

	email, err := svc.GetEmailByID(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "user5@example.com", email)
}

func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, _, sessionRepo, _, _, _, _, _ := setupAuthTest(t)

		sessionRepo.On("DeleteSessionByHash", ctx, mock.AnythingOfType("string")).Return(nil)

		err := svc.Logout(ctx, "valid-refresh-token")
		require.NoError(t, err)
		sessionRepo.AssertExpectations(t)
	})

	t.Run("empty refresh token", func(t *testing.T) {
		svc, _, _, _, _, _, _, _ := setupAuthTest(t)

		err := svc.Logout(ctx, "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrInvalidToken))
	})
}

func TestAuthService_Refresh(t *testing.T) {
	ctx := context.Background()

	t.Run("success rotation", func(t *testing.T) {
		svc, _, sessionRepo, _, _, _, txManager, _ := setupAuthTest(t)

		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		sessionRepo.On("ConsumeSessionByHash", ctx, mock.AnythingOfType("string")).Return(int64(77), nil)
		sessionRepo.On("CreateSession", ctx, mock.MatchedBy(func(s *model.Session) bool {
			return s.UserId == 77 && !s.Revoked
		})).Return(nil)

		res, err := svc.Refresh(ctx, "valid-refresh-token")
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.Equal(t, 900, res.ExpiresIn)

		sessionRepo.AssertExpectations(t)
		txManager.AssertExpectations(t)
	})

	t.Run("empty token", func(t *testing.T) {
		svc, _, _, _, _, _, _, _ := setupAuthTest(t)

		res, err := svc.Refresh(ctx, "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrInvalidToken))
		assert.Nil(t, res)
	})

	t.Run("session expired or consumed", func(t *testing.T) {
		svc, _, sessionRepo, _, _, _, txManager, _ := setupAuthTest(t)

		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		sessionRepo.On("ConsumeSessionByHash", ctx, mock.AnythingOfType("string")).Return(int64(0), apperrors.ErrTokenNotFound)

		res, err := svc.Refresh(ctx, "expired-token")
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestAuthService_TOTP(t *testing.T) {
	ctx := context.Background()

	t.Run("EnableTOTP generates secret and qr", func(t *testing.T) {
		svc, _, _, totpRepo, _, _, _, _ := setupAuthTest(t)

		totpRepo.On("UpdateTOTPSecret", ctx, int64(10), mock.AnythingOfType("string"), false).Return(nil)

		secret, qr, err := svc.EnableTOTP(ctx, 10, "totp@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, secret)
		assert.NotEmpty(t, qr)
		totpRepo.AssertExpectations(t)
	})

	t.Run("VerifyTOTP - valid code", func(t *testing.T) {
		svc, _, _, totpRepo, _, _, _, _ := setupAuthTest(t)

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "SmartRun",
			AccountName: "totp@example.com",
		})
		require.NoError(t, err)
		secret := key.Secret()

		code, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)

		totpRepo.On("GetTOTPSecret", ctx, int64(10)).Return(secret, nil)
		totpRepo.On("UpdateTOTPSecret", ctx, int64(10), secret, true).Return(nil)

		valid, err := svc.VerifyTOTP(ctx, 10, code)
		require.NoError(t, err)
		assert.True(t, valid)
		totpRepo.AssertExpectations(t)
	})

	t.Run("VerifyTOTP - invalid code", func(t *testing.T) {
		svc, _, _, totpRepo, _, _, _, _ := setupAuthTest(t)

		totpRepo.On("GetTOTPSecret", ctx, int64(10)).Return("JBSWY3DPEHPK3PXP", nil)

		valid, err := svc.VerifyTOTP(ctx, 10, "000000")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("IsTOTPEnabled", func(t *testing.T) {
		svc, _, _, totpRepo, _, _, _, _ := setupAuthTest(t)

		totpRepo.On("IsTOTPEnabled", ctx, int64(10)).Return(true, nil)

		enabled, err := svc.IsTOTPEnabled(ctx, 10)
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("IssueTokensAfter2FA", func(t *testing.T) {
		svc, _, sessionRepo, _, _, _, txManager, _ := setupAuthTest(t)

		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		sessionRepo.On("CreateSession", ctx, mock.MatchedBy(func(s *model.Session) bool {
			return s.UserId == 10 && !s.Revoked
		})).Return(nil)

		res, err := svc.IssueTokensAfter2FA(ctx, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.Equal(t, 900, res.ExpiresIn)

		sessionRepo.AssertExpectations(t)
		txManager.AssertExpectations(t)
	})
}

func TestAuthService_PasswordReset(t *testing.T) {
	ctx := context.Background()

	t.Run("RequestPasswordReset success", func(t *testing.T) {
		svc, userRepo, _, _, pwResetRepo, _, _, publisher := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "reset@example.com").Return(&model.User{
			ID:    55,
			Email: "reset@example.com",
			Name:  "Reset User",
		}, nil)
		pwResetRepo.On("CreateResetToken", ctx, int64(55), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
		publisher.On("PublishPasswordReset", mock.Anything, mock.MatchedBy(func(e broker.PasswordResetEvent) bool {
			return e.To == "reset@example.com" && e.UserName == "Reset User" && e.ResetLink != ""
		})).Return(nil)

		err := svc.RequestPasswordReset(ctx, "reset@example.com")
		require.NoError(t, err)

		userRepo.AssertExpectations(t)
		pwResetRepo.AssertExpectations(t)
		publisher.AssertExpectations(t)
	})

	t.Run("RequestPasswordReset user not found", func(t *testing.T) {
		svc, userRepo, _, _, _, _, _, _ := setupAuthTest(t)

		userRepo.On("GetUserByEmail", ctx, "ghost@example.com").Return(nil, apperrors.ErrUserNotFound)

		err := svc.RequestPasswordReset(ctx, "ghost@example.com")
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrUserNotFound))
	})

	t.Run("ValidateResetToken - valid", func(t *testing.T) {
		svc, _, _, _, pwResetRepo, _, _, _ := setupAuthTest(t)

		pwResetRepo.On("FindResetByTokenHash", ctx, mock.AnythingOfType("string")).Return(int64(55), false, nil)

		userID, err := svc.ValidateResetToken(ctx, "some-token")
		require.NoError(t, err)
		assert.Equal(t, int64(55), userID)
	})

	t.Run("ValidateResetToken - already used", func(t *testing.T) {
		svc, _, _, _, pwResetRepo, _, _, _ := setupAuthTest(t)

		pwResetRepo.On("FindResetByTokenHash", ctx, mock.AnythingOfType("string")).Return(int64(55), true, nil)

		userID, err := svc.ValidateResetToken(ctx, "some-token")
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperrors.ErrTokenAlreadyUsed))
		assert.Equal(t, int64(0), userID)
	})

	t.Run("PerformPasswordReset success", func(t *testing.T) {
		svc, userRepo, sessionRepo, _, pwResetRepo, _, txManager, _ := setupAuthTest(t)

		userRepo.On("GetUserByID", ctx, int64(55)).Return(&model.User{
			ID:    55,
			Email: "reset@example.com",
		}, nil)
		txManager.On("WithTransaction", ctx, mock.Anything).Return(nil)
		pwResetRepo.On("ConsumeResetToken", ctx, mock.AnythingOfType("string"), int64(55)).Return(nil)
		userRepo.On("UpdatePassword", ctx, int64(55), mock.AnythingOfType("string")).Return(nil)
		sessionRepo.On("DeleteAllSessionsForUser", ctx, int64(55)).Return(nil)
		userRepo.On("InvalidateUserCache", ctx, int64(55), "reset@example.com").Return()

		err := svc.PerformPasswordReset(ctx, 55, "new_secure_password_99", "reset-token")
		require.NoError(t, err)

		userRepo.AssertExpectations(t)
		pwResetRepo.AssertExpectations(t)
		sessionRepo.AssertExpectations(t)
		txManager.AssertExpectations(t)
	})
}

func TestAuthService_GetSessionByHash(t *testing.T) {
	ctx := context.Background()
	svc, _, sessionRepo, _, _, _, _, _ := setupAuthTest(t)

	expectedSession := &model.Session{
		UserId:      10,
		RefreshHash: "hash123",
	}
	sessionRepo.On("FindSessionByHash", ctx, "hash123").Return(expectedSession, nil)

	s, err := svc.GetSessionByHash(ctx, "hash123")
	require.NoError(t, err)
	assert.Equal(t, expectedSession, s)
}
