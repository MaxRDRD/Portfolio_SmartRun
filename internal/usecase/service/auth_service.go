package service

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/config"
	"SmartRun/internal/dto"
	appLogger "SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/internal/repository_impl/postgres"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/go-playground/validator"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResult, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error)
	UpdateUser(ctx context.Context, userID int64, req dto.UpdateUserRequest) (*dto.UserResponse, error)
	GetEmailByID(ctx context.Context, id int64) (string, error)
	VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error)
	IssueTokensAfter2FA(ctx context.Context, userID int64) (*dto.AuthResult, error)
	IsTOTPEnabled(ctx context.Context, userID int64) (bool, error)
	Refresh(ctx context.Context, refreshToken string) (*dto.AuthResult, error)
	EnableTOTP(ctx context.Context, userID int64, email string) (string, []byte, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ValidateResetToken(ctx context.Context, token string) (int64, error) // возвращает userID если токен валиден
	PerformPasswordReset(ctx context.Context, userID int64, newPassword string, resetToken string) error
	GetSessionByHash(ctx context.Context, tokenHash string) (*model.Session, error) // для Verify2FA
}

type authService struct {
	userRepo          repository.UserRepository
	sessionRepo       repository.SessionRepository
	totpRepo          repository.TotpRepository
	passwordResetRepo repository.PasswordResetRepository
	emailService      EmailService
	cfg               config.AuthConfig
	validate          *validator.Validate
	txManager         repository.TxManager
}

func NewUserService(userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	totpRepo repository.TotpRepository,
	passwordResetRepo repository.PasswordResetRepository,
	emailService EmailService,
	cfg config.AuthConfig,
	txManager repository.TxManager,
	validator *validator.Validate) AuthService {
	return &authService{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		totpRepo:          totpRepo,
		passwordResetRepo: passwordResetRepo,
		emailService:      emailService,
		cfg:               cfg,
		validate:          validator,
		txManager:         txManager,
	}
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResult, error) {
	log := appLogger.FromContext(ctx)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	if err := s.validate.Struct(req); err != nil {
		log.Warn("auth service: register validation failed", "email", req.Email, "error", err)
		return nil, err
	}

	var result *dto.AuthResult

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		// все репозитории теперь видят tx из контекста автоматически

		_, err := s.userRepo.GetUserByEmail(ctx, req.Email)
		if err == nil {
			return my_errors.ErrUserAlreadyExists
		}
		if !errors.Is(err, my_errors.ErrUserNotFound) {
			return err
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hp := string(hash)
		prefix := hp
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		log.Debug("auth service: register password hashed", "email", req.Email, "hash_prefix", prefix, "hash_len", len(hp), "password_len", len(req.Password))

		user := &model.User{
			Name:     req.Name,
			Email:    req.Email,
			Password: string(hash),
			Gender:   strings.ToLower(strings.TrimSpace(req.Gender)),
		}
		if req.Age != nil {
			user.Age = *req.Age
		}
		if req.WeightKg != nil {
			user.WeightKg = *req.WeightKg
		}
		if req.HeightCm != nil {
			user.HeightCm = *req.HeightCm
		}
		if req.RestingHR != nil {
			user.RestingHR = *req.RestingHR
		}
		if req.MaxHR != nil {
			user.MaxHR = *req.MaxHR
		}
		if req.WeeklyRuns != nil {
			user.WeeklyRuns = *req.WeeklyRuns
		}
		if req.ThresholdPace != nil {
			user.ThresholdPace = *req.ThresholdPace
		}

		if err = s.userRepo.CreateUser(ctx, user); err != nil {
			return err
		}

		refreshToken, session, err := s.createSession(user.ID)
		if err != nil {
			return err
		}

		if err = s.sessionRepo.CreateSession(ctx, session); err != nil {
			return err
		}

		accessToken, err := auth.GenerateAccessToken(user.ID, s.cfg.AccessTokenTTL)
		if err != nil {
			return err
		}

		result = &dto.AuthResult{
			AccessToken:  accessToken,
			ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
			RefreshToken: refreshToken,
			User: &dto.UserResponse{
				ID:            user.ID,
				Email:         user.Email,
				Name:          user.Name,
				TOTPEnabled:   false,
				Gender:        user.Gender,
				Age:           user.Age,
				WeightKg:      user.WeightKg,
				HeightCm:      user.HeightCm,
				RestingHR:     user.RestingHR,
				MaxHR:         user.MaxHR,
				WeeklyRuns:    user.WeeklyRuns,
				ThresholdPace: user.ThresholdPace,
			},
		}

		return nil
	})

	if err != nil {
		log.Error("auth service: register failed", "email", req.Email, "error", err)
	} else if result != nil && result.User != nil {
		log.Info("auth service: register success", "user_id", result.User.ID, "email", result.User.Email)
	}
	return result, err
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResult, error) {
	log := appLogger.FromContext(ctx)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	if err := s.validate.Struct(req); err != nil {
		log.Warn("auth service: login validation failed", "email", req.Email, "password_len", len(req.Password), "error", err)
		return nil, err
	}

	log.Info("auth service: login attempt", "email", req.Email)
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		log.Warn("auth service: login lookup failed", "email", req.Email, "error", err)
		return nil, my_errors.ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// dev-диагностика: не печатаем хэш целиком
		prefix := user.Password
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		log.Warn("auth service: login password mismatch", "email", req.Email, "hash_prefix", prefix, "hash_len", len(user.Password), "password_len", len(req.Password), "error", err)
		return nil, my_errors.ErrInvalidCredentials
	}

	var result *dto.AuthResult

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.sessionRepo.DeleteAllSessionsForUser(ctx, user.ID); err != nil {
			return err
		}

		refreshToken, session, err := s.createSession(user.ID)
		if err != nil {
			return err
		}

		if err = s.sessionRepo.CreateSession(ctx, session); err != nil {
			return err
		}

		enabled, err := s.totpRepo.IsTOTPEnabled(ctx, user.ID)
		if err != nil {
			return err
		}

		accessToken, err := auth.GenerateAccessToken(user.ID, s.cfg.AccessTokenTTL)
		if err != nil {
			return err
		}

		result = &dto.AuthResult{
			AccessToken:  accessToken,
			ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
			RefreshToken: refreshToken,
			User: &dto.UserResponse{
				ID:            user.ID,
				Email:         user.Email,
				Name:          user.Name,
				TOTPEnabled:   enabled,
				Gender:        user.Gender,
				Age:           user.Age,
				WeightKg:      user.WeightKg,
				HeightCm:      user.HeightCm,
				RestingHR:     user.RestingHR,
				MaxHR:         user.MaxHR,
				WeeklyRuns:    user.WeeklyRuns,
				ThresholdPace: user.ThresholdPace,
			},
		}
		return nil
	})

	if err != nil {
		log.Error("auth service: login failed", "email", req.Email, "error", err)
	} else if result != nil && result.User != nil {
		log.Info("auth service: login success", "user_id", result.User.ID, "email", result.User.Email)
	}
	return result, err

}

func (s *authService) GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		Gender:        user.Gender,
		Age:           user.Age,
		WeightKg:      user.WeightKg,
		HeightCm:      user.HeightCm,
		RestingHR:     user.RestingHR,
		MaxHR:         user.MaxHR,
		WeeklyRuns:    user.WeeklyRuns,
		ThresholdPace: user.ThresholdPace,
	}, nil
}

func (s *authService) UpdateUser(ctx context.Context, userID int64, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	oldEmail := user.Email

	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
	}
	if req.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*req.Email))
		if newEmail == "" {
			return nil, fmt.Errorf("email cannot be empty")
		}
		if newEmail != oldEmail {
			existing, getErr := s.userRepo.GetUserByEmail(ctx, newEmail)
			if getErr == nil && existing.ID != userID {
				return nil, my_errors.ErrUserAlreadyExists
			}
			if getErr != nil && !errors.Is(getErr, my_errors.ErrUserNotFound) {
				return nil, getErr
			}
		}
		user.Email = newEmail
	}
	if req.Gender != nil {
		user.Gender = strings.ToLower(strings.TrimSpace(*req.Gender))
	}
	if req.Age != nil {
		user.Age = *req.Age
	}
	if req.WeightKg != nil {
		user.WeightKg = *req.WeightKg
	}
	if req.HeightCm != nil {
		user.HeightCm = *req.HeightCm
	}
	if req.RestingHR != nil {
		user.RestingHR = *req.RestingHR
	}
	if req.MaxHR != nil {
		user.MaxHR = *req.MaxHR
	}
	if req.WeeklyRuns != nil {
		user.WeeklyRuns = *req.WeeklyRuns
	}
	if req.ThresholdPace != nil {
		user.ThresholdPace = *req.ThresholdPace
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	s.userRepo.InvalidateUserCache(ctx, userID, oldEmail)
	if user.Email != oldEmail {
		s.userRepo.InvalidateUserCache(ctx, userID, user.Email)
	}

	return &dto.UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		Gender:        user.Gender,
		Age:           user.Age,
		WeightKg:      user.WeightKg,
		HeightCm:      user.HeightCm,
		RestingHR:     user.RestingHR,
		MaxHR:         user.MaxHR,
		WeeklyRuns:    user.WeeklyRuns,
		ThresholdPace: user.ThresholdPace,
	}, nil
}

func (s *authService) GetEmailByID(ctx context.Context, id int64) (string, error) {
	email, err := s.userRepo.GetEmailByID(ctx, id)
	return email, err
}

func (s *authService) DeleteRefreshToken(ctx context.Context, refresh_hash string) error {
	err := s.sessionRepo.DeleteSessionByHash(ctx, refresh_hash)
	return err
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return my_errors.ErrInvalidToken // уже разлогинены
	}
	hash := auth.HashToken(refreshToken)
	return s.sessionRepo.DeleteSessionByHash(ctx, hash)
}

// Refresh обновляет access-токен и выполняет ротацию refresh-токена
func (s *authService) Refresh(ctx context.Context, refreshToken string) (*dto.AuthResult, error) {
	if refreshToken == "" {
		return nil, my_errors.ErrInvalidToken
	}

	refreshHash := auth.HashToken(refreshToken)

	session, err := s.sessionRepo.FindSessionByHash(ctx, refreshHash)
	if err != nil {
		return nil, err
	}

	var result *dto.AuthResult

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.sessionRepo.DeleteSessionByHash(ctx, refreshHash); err != nil {
			return err
		}

		newRefresh, newSession, err := s.createSession(session.UserId)
		if err != nil {
			return err
		}

		if err = s.sessionRepo.CreateSession(ctx, newSession); err != nil {
			return err
		}

		newAccess, err := auth.GenerateAccessToken(session.UserId, s.cfg.AccessTokenTTL)
		if err != nil {
			return err
		}

		result = &dto.AuthResult{
			AccessToken:  newAccess,
			ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
			RefreshToken: newRefresh,
		}
		return nil
	})

	return result, err
}

func (s *authService) EnableTOTP(ctx context.Context, userID int64, email string) (string, []byte, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "SmartRun",
		AccountName: email,
	})
	if err != nil {
		return "", nil, err
	}

	// Сохраняем в БД
	err = s.totpRepo.UpdateTOTPSecret(ctx, userID, key.Secret(), true)
	if err != nil {
		return "", nil, err
	}

	// Генерация QR
	qr, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return "", nil, err
	}

	return key.Secret(), qr, nil // Возвращаем для ручной настройки, если QR не сработает
}

func (s *authService) VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error) {
	secret, err := s.totpRepo.GetTOTPSecret(ctx, userID)
	if err != nil {
		return false, err
	}
	return totp.Validate(code, secret), nil
}

func (s *authService) IsTOTPEnabled(ctx context.Context, userID int64) (bool, error) {
	TOTPEnabled, err := s.totpRepo.IsTOTPEnabled(ctx, userID)
	if err != nil {
		return false, err
	}
	return TOTPEnabled, nil
}

func (s *authService) IssueTokensAfter2FA(ctx context.Context, userID int64) (*dto.AuthResult, error) {

	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // Автоматический откат, если не commit

	refreshToken, session, err := s.createSession(userID)
	if err != nil {
		return nil, err
	}

	sessionRepoTx := postgres.NewSessionRepository(tx)
	err = sessionRepoTx.CreateSession(ctx, session)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	accessToken, err := auth.GenerateAccessToken(userID, s.cfg.AccessTokenTTL)
	expiresIn := int(s.cfg.AccessTokenTTL.Seconds()) // 900

	res := &dto.AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
	}

	return res, nil
}

func (s *authService) createSession(userID int64) (string, *model.Session, error) {

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", nil, err
	}

	refreshHash := auth.HashToken(refreshToken)

	session := &model.Session{
		UserId:      userID,
		RefreshHash: refreshHash,
		ExpiresAt:   time.Now().UTC().Add(s.cfg.RefreshTokenTTL),
		Revoked:     false,
	}

	return refreshToken, session, nil
}

func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return my_errors.ErrUserNotFound
	}

	token, err := auth.GenerateRefreshToken()
	if err != nil {
		return err
	}
	tokenHash := auth.HashToken(token)

	expiresAt := time.Now().UTC().Add(60 * time.Minute) // 1 час — типично

	err = s.passwordResetRepo.CreateResetToken(ctx, user.ID, tokenHash, expiresAt)
	if err != nil {
		return err
	}

	publicURL := s.cfg.PublicURL
	if publicURL == "" {
		publicURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", publicURL, token)
	// Здесь вызов сервиса отправки почты
	return s.emailService.SendPasswordResetEmail(ctx, user.Email, resetLink, user.Name)
}

func (s *authService) ValidateResetToken(ctx context.Context, token string) (int64, error) {
	hash := auth.HashToken(token)
	userID, used, err := s.passwordResetRepo.FindResetByTokenHash(ctx, hash)
	if err != nil {
		return 0, err
	}
	if used {
		return 0, my_errors.ErrTokenAlreadyUsed
	}
	return userID, nil
}

func (s *authService) PerformPasswordReset(ctx context.Context, userID int64, newPassword string, resetToken string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		hash := auth.HashToken(resetToken)

		uid, used, err := s.passwordResetRepo.FindResetByTokenHash(ctx, hash)
		if err != nil {
			return err
		}
		if uid != userID || used {
			return my_errors.ErrInvalidToken
		}

		passHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		if err = s.userRepo.UpdatePassword(ctx, userID, string(passHash)); err != nil {
			return err
		}

		if err = s.passwordResetRepo.MarkAsUsed(ctx, hash); err != nil {
			return err
		}

		// Опционально, но рекомендуется
		return s.sessionRepo.DeleteAllSessionsForUser(ctx, userID)
	})
	if err != nil {
		return err
	}

	s.userRepo.InvalidateUserCache(ctx, userID, user.Email)
	return nil
}

// GetSessionByHash — вспомогательный метод для Verify2FA (получить сессию по хешу refresh token)
func (s *authService) GetSessionByHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	return s.sessionRepo.FindSessionByHash(ctx, tokenHash)
}
