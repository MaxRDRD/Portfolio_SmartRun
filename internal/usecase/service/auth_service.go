package service

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/config"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/internal/repository_impl/postgres"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"

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
	GetEmailByID(ctx context.Context, id int64) (string, error)
	VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error)
	IssueTokensAfter2FA(ctx context.Context, userID int64) (*dto.AuthResult, error)
	IsTOTPEnabled(ctx context.Context, userID int64) (bool, error)
	Refresh(ctx context.Context, refreshToken string) (*dto.AuthResult, error)
	EnableTOTP(ctx context.Context, userID int64, email string) (string, []byte, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ValidateResetToken(ctx context.Context, token string) (int64, error) // возвращает userID если токен валиден
	PerformPasswordReset(ctx context.Context, userID int64, newPassword string, resetToken string) error
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
	txManager repository.TxManager) AuthService {
	return &authService{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		totpRepo:          totpRepo,
		passwordResetRepo: passwordResetRepo,
		emailService:      emailService,
		cfg:               cfg,
		validate:          validator.New(),
		txManager:         txManager,
	}
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResult, error) {
	user := &model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	}
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	// 1. Проверяем, существует ли email
	_, err := s.userRepo.GetUserByEmail(ctx, user.Email)

	if err == nil {
		return nil, my_errors.ErrUserAlreadyExists
	}

	if !errors.Is(err, my_errors.ErrUserNotFound) {
		return nil, err
	}

	// 2. Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user.Password = string(hash)

	// 3. Сохраняем
	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // безопасный откат, если не будет Commit

	userRepoTx := postgres.NewUserRepository(tx)
	err = userRepoTx.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	refreshToken, session, err := s.createSession(user.ID)
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

	accessToken, err := auth.GenerateAccessToken(user.ID, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	userResp := &dto.UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		TOTPEnabled: false,
	}

	res := &dto.AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		User:         userResp,
	}

	return res, nil
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResult, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, my_errors.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(req.Password),
	)
	if err != nil {
		return nil, my_errors.ErrInvalidCredentials
	}
	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // Автоматический откат, если не commit

	refreshToken, session, err := s.createSession(user.ID)
	if err != nil {
		return nil, err
	}

	sessionRepoTx := postgres.NewSessionRepository(tx)
	err = sessionRepoTx.DeleteAllSessionsForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	err = sessionRepoTx.CreateSession(ctx, session)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, s.cfg.AccessTokenTTL)
	expiresIn := int(s.cfg.AccessTokenTTL.Seconds()) // 900

	enabled, err := s.IsTOTPEnabled(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("check totp enabled: %w", err)
	}

	userResp := &dto.UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		TOTPEnabled: enabled,
	}

	res := &dto.AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
		User:         userResp,
	}

	return res, nil

}

func (s *authService) GetUserByID(ctx context.Context, id int64) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.UserResponse{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
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

	// Вычисляем хеш входящего refresh-токена
	refreshHash := auth.HashToken(refreshToken)

	// Ищем существующую сессию
	session, err := s.sessionRepo.FindSessionByHash(ctx, refreshHash)
	if err != nil {
		// Здесь уже может быть ErrInvalidToken или ErrTokenNotFound
		return nil, fmt.Errorf("find session: %w", err)
	}

	// Дополнительная проверка (на всякий случай, если FindSessionByHash её пропустил)
	if session.Revoked || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, my_errors.ErrInvalidToken
	}

	// Генерируем новый access token
	newAccessToken, err := auth.GenerateAccessToken(session.UserId, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Генерируем новый refresh token (ротация)
	newRefreshToken, newSession, err := s.createSession(session.UserId)

	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	repoTx := postgres.NewSessionRepository(tx)

	if err := repoTx.DeleteSessionByHash(ctx, refreshHash); err != nil {
		return nil, err
	}

	if err := repoTx.CreateSession(ctx, newSession); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Вариант Б (альтернатива): просто обновляем существующую запись
	// err = s.repo.UpdateSessionHash(ctx, refreshHash, newRefreshHash, newExpiresAt)
	// — но требует дополнительного метода в repository

	res := &dto.AuthResult{
		AccessToken:  newAccessToken,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: newRefreshToken,
	}

	return res, nil
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

	resetLink := fmt.Sprintf("https://your-app.com/reset-password?token=%s", token)
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
	tx, err := s.txManager.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Проверяем токен ещё раз в транзакции
	hash := auth.HashToken(resetToken)
	repo := postgres.NewPasswordResetRepository(tx) // tx-aware
	uid, used, err := repo.FindResetByTokenHash(ctx, hash)
	if err != nil || uid != userID || used {
		return my_errors.ErrInvalidToken
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = s.userRepo.UpdatePassword(ctx, userID, string(hashPass))
	if err != nil {
		return err
	}

	// Помечаем токен использованным ИЛИ удаляем
	err = repo.MarkAsUsed(ctx, hash)
	// Опционально: инвалидируем все сессии
	sessionRepo := postgres.NewSessionRepository(tx)
	sessionRepo.DeleteAllSessionsForUser(ctx, userID)

	return tx.Commit(ctx)
}
