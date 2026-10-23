package service

import (
	"SmartRun/internal/adapter/webauthn"
	"SmartRun/internal/auth"
	"SmartRun/internal/cache"
	"SmartRun/internal/config"
	"SmartRun/internal/dto"
	appLogger "SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/apperrors"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	webauthnpkg "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type PasskeyService interface {
	// Регистрация нового Passkey для авторизованного пользователя
	BeginRegistration(ctx context.Context, userID int64) (*protocol.CredentialCreation, error)
	FinishRegistration(ctx context.Context, userID int64, body io.Reader, deviceName string) (*model.Passkey, error)

	// Аутентификация / Вход по Passkey
	BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, string, error)
	FinishLogin(ctx context.Context, sessionID string, body io.Reader) (*dto.AuthResult, error)

	// Управление ключами
	GetUserPasskeys(ctx context.Context, userID int64) ([]model.Passkey, error)
	DeletePasskey(ctx context.Context, id int64, userID int64) error
}

type passkeyService struct {
	userRepo    repository.UserRepository
	passkeyRepo repository.PasskeyRepository
	sessionRepo repository.SessionRepository
	totpRepo    repository.TotpRepository
	txManager   repository.TxManager
	cache       cache.Cache
	webAuthn    *webauthnpkg.WebAuthn
	cfg         config.AuthConfig
}

var _ PasskeyService = (*passkeyService)(nil)

func NewPasskeyService(
	userRepo repository.UserRepository,
	passkeyRepo repository.PasskeyRepository,
	sessionRepo repository.SessionRepository,
	totpRepo repository.TotpRepository,
	txManager repository.TxManager,
	cache cache.Cache,
	webAuthn *webauthnpkg.WebAuthn,
	cfg config.AuthConfig,
) PasskeyService {
	return &passkeyService{
		userRepo:    userRepo,
		passkeyRepo: passkeyRepo,
		sessionRepo: sessionRepo,
		totpRepo:    totpRepo,
		txManager:   txManager,
		cache:       cache,
		webAuthn:    webAuthn,
		cfg:         cfg,
	}
}

// 1. BeginRegistration начинает церемонию создания нового Passkey
func (s *passkeyService) BeginRegistration(ctx context.Context, userID int64) (*protocol.CredentialCreation, error) {
	log := appLogger.FromContext(ctx)

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("BeginRegistration: %w", err)
	}

	passkeys, err := s.passkeyRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("BeginRegistration get passkeys: %w", err)
	}

	webAuthnUser := adapter.NewWebAuthnUser(user, passkeys)

	// Настройка критериев аутентификатора: отдаем приоритет Passkeys (resident keys)
	authSelection := protocol.AuthenticatorSelection{
		ResidentKey:      protocol.ResidentKeyRequirementPreferred,
		UserVerification: protocol.VerificationPreferred,
	}

	creation, sessionData, err := s.webAuthn.BeginRegistration(
		webAuthnUser,
		webauthnpkg.WithAuthenticatorSelection(authSelection),
	)
	if err != nil {
		log.Error("passkey service: failed to begin registration", "user_id", userID, "error", err)
		return nil, fmt.Errorf("webauthn.BeginRegistration: %w", err)
	}

	sessionBytes, err := json.Marshal(sessionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sessionData: %w", err)
	}

	cacheKey := fmt.Sprintf("passkey:reg:%d", userID)
	if err := s.cache.Set(ctx, cacheKey, string(sessionBytes), 5*time.Minute); err != nil {
		log.Warn("passkey service: failed to cache registration session", "user_id", userID, "error", err)
	}

	log.Info("passkey service: registration started", "user_id", userID)
	return creation, nil
}

// 2. FinishRegistration проверяет ответ от браузера/аутентификатора и сохраняет ключ в БД
func (s *passkeyService) FinishRegistration(ctx context.Context, userID int64, body io.Reader, deviceName string) (*model.Passkey, error) {
	log := appLogger.FromContext(ctx)

	cacheKey := fmt.Sprintf("passkey:reg:%d", userID)
	sessionStr, err := s.cache.Get(ctx, cacheKey)
	if err != nil || sessionStr == "" {
		return nil, apperrors.ErrPasskeySessionExpired
	}
	_ = s.cache.Del(ctx, cacheKey)

	var sessionData webauthnpkg.SessionData
	if err := json.Unmarshal([]byte(sessionStr), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sessionData: %w", err)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration: %w", err)
	}

	passkeys, err := s.passkeyRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("FinishRegistration get passkeys: %w", err)
	}

	webAuthnUser := adapter.NewWebAuthnUser(user, passkeys)

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(body)
	if err != nil {
		log.Warn("passkey service: failed to parse credential creation response", "user_id", userID, "error", err)
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidData, err)
	}

	cred, err := s.webAuthn.CreateCredential(webAuthnUser, sessionData, parsedResponse)
	if err != nil {
		log.Warn("passkey service: failed to verify credential creation", "user_id", userID, "error", err)
		return nil, fmt.Errorf("%w: %v", apperrors.ErrPasskeyVerification, err)
	}

	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}

	if deviceName == "" {
		deviceName = "Passkey Device"
	}

	newPasskey := &model.Passkey{
		UserID:          userID,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		Transports:      transports,
		BackupState:     cred.Flags.BackupState,
		DeviceName:      deviceName,
	}

	if err := s.passkeyRepo.Create(ctx, newPasskey); err != nil {
		log.Error("passkey service: failed to save passkey in db", "user_id", userID, "error", err)
		return nil, fmt.Errorf("save passkey: %w", err)
	}

	log.Info("passkey service: passkey registered successfully", "user_id", userID, "passkey_id", newPasskey.ID)
	return newPasskey, nil
}

// 3. BeginLogin начинает процесс аутентификации (по email или 1-click discoverable passkey)
func (s *passkeyService) BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, string, error) {
	log := appLogger.FromContext(ctx)

	var assertion *protocol.CredentialAssertion
	var sessionData *webauthnpkg.SessionData
	var err error

	if email != "" {
		user, err := s.userRepo.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, "", apperrors.ErrUserNotFound
		}

		passkeys, err := s.passkeyRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			return nil, "", fmt.Errorf("BeginLogin get passkeys: %w", err)
		}

		if len(passkeys) == 0 {
			return nil, "", apperrors.ErrPasskeyNotFound
		}

		webAuthnUser := adapter.NewWebAuthnUser(user, passkeys)
		assertion, sessionData, err = s.webAuthn.BeginLogin(webAuthnUser)
		if err != nil {
			log.Error("passkey service: failed to begin login", "email", email, "error", err)
			return nil, "", fmt.Errorf("webauthn.BeginLogin: %w", err)
		}
	} else {
		// Discoverable passkey login (без ввода email)
		assertion, sessionData, err = s.webAuthn.BeginDiscoverableLogin()
		if err != nil {
			log.Error("passkey service: failed to begin discoverable login", "error", err)
			return nil, "", fmt.Errorf("webauthn.BeginDiscoverableLogin: %w", err)
		}
	}

	sessionID := uuid.New().String()
	sessionBytes, err := json.Marshal(sessionData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal sessionData: %w", err)
	}

	cacheKey := fmt.Sprintf("passkey:login:%s", sessionID)
	if err := s.cache.Set(ctx, cacheKey, string(sessionBytes), 5*time.Minute); err != nil {
		log.Warn("passkey service: failed to cache login session", "session_id", sessionID, "error", err)
	}

	log.Info("passkey service: login ceremony started", "session_id", sessionID, "has_email", email != "")
	return assertion, sessionID, nil
}

// 4. FinishLogin проверяет криптографическую подпись и выдает JWT + refresh-сессию
func (s *passkeyService) FinishLogin(ctx context.Context, sessionID string, body io.Reader) (*dto.AuthResult, error) {
	log := appLogger.FromContext(ctx)

	if sessionID == "" {
		return nil, apperrors.ErrPasskeySessionExpired
	}

	cacheKey := fmt.Sprintf("passkey:login:%s", sessionID)
	sessionStr, err := s.cache.Get(ctx, cacheKey)
	if err != nil || sessionStr == "" {
		return nil, apperrors.ErrPasskeySessionExpired
	}
	_ = s.cache.Del(ctx, cacheKey)

	var sessionData webauthnpkg.SessionData
	if err := json.Unmarshal([]byte(sessionStr), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sessionData: %w", err)
	}

	parsedAssertion, err := protocol.ParseCredentialRequestResponseBody(body)
	if err != nil {
		log.Warn("passkey service: failed to parse credential request response", "error", err)
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidData, err)
	}

	var cred *webauthnpkg.Credential
	var user *model.User
	var passkey *model.Passkey

	if len(sessionData.UserID) == 0 {
		// 1. Discoverable login (1-click passkey): пользователь определяется аутентификатором
		handler := func(rawID, userHandle []byte) (webauthnpkg.User, error) {
			pk, pkErr := s.passkeyRepo.GetByCredentialID(ctx, rawID)
			if pkErr != nil {
				return nil, pkErr
			}
			u, uErr := s.userRepo.GetUserByID(ctx, pk.UserID)
			if uErr != nil {
				return nil, uErr
			}
			pks, pksErr := s.passkeyRepo.GetByUserID(ctx, u.ID)
			if pksErr != nil {
				return nil, pksErr
			}
			return adapter.NewWebAuthnUser(u, pks), nil
		}

		authedUser, validatedCred, valErr := s.webAuthn.ValidatePasskeyLogin(handler, sessionData, parsedAssertion)
		if valErr != nil {
			log.Warn("passkey service: discoverable login validation failed", "error", valErr)
			return nil, fmt.Errorf("%w: %v", apperrors.ErrPasskeyVerification, valErr)
		}

		cred = validatedCred
		userAdapter, ok := authedUser.(*adapter.WebAuthnUserAdapter)
		if !ok {
			return nil, fmt.Errorf("unexpected user adapter type")
		}
		user = userAdapter.User
		passkey, err = s.passkeyRepo.GetByCredentialID(ctx, cred.ID)
		if err != nil {
			return nil, fmt.Errorf("lookup authenticated passkey: %w", err)
		}
	} else {
		// 2. Non-discoverable login (по email): пользователь уже известен
		passkey, err = s.passkeyRepo.GetByCredentialID(ctx, parsedAssertion.RawID)
		if err != nil {
			if errors.Is(err, apperrors.ErrPasskeyNotFound) {
				return nil, apperrors.ErrInvalidCredentials
			}
			return nil, fmt.Errorf("lookup passkey by credential id: %w", err)
		}

		user, err = s.userRepo.GetUserByID(ctx, passkey.UserID)
		if err != nil {
			return nil, fmt.Errorf("lookup user: %w", err)
		}

		passkeys, err := s.passkeyRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("get user passkeys: %w", err)
		}

		webAuthnUser := adapter.NewWebAuthnUser(user, passkeys)
		cred, err = s.webAuthn.ValidateLogin(webAuthnUser, sessionData, parsedAssertion)
		if err != nil {
			log.Warn("passkey service: login validation failed", "user_id", user.ID, "error", err)
			return nil, fmt.Errorf("%w: %v", apperrors.ErrPasskeyVerification, err)
		}
	}

	// Защита от клонирования ключа: проверка предупреждения библиотеки
	if cred.Authenticator.CloneWarning {
		log.Warn("passkey service: possible cloned authenticator detected", "user_id", user.ID, "passkey_id", passkey.ID)
		return nil, fmt.Errorf("%w: cloned authenticator detected", apperrors.ErrPasskeyVerification)
	}

	// Атомарный выпуск сессии и обновление счетчика использования
	var accessToken string
	var refreshToken string

	createSessionFn := func(ctx context.Context) error {
		// Удаляем старые сессии пользователя (политика single-session, как в AuthService)
		if err := s.sessionRepo.DeleteAllSessionsForUser(ctx, user.ID); err != nil {
			return fmt.Errorf("delete old sessions: %w", err)
		}

		// Обновляем SignCount и время использования в БД
		if err := s.passkeyRepo.UpdateSignCountAndLastUsed(ctx, passkey.ID, cred.Authenticator.SignCount); err != nil {
			log.Warn("passkey service: failed to update sign count", "passkey_id", passkey.ID, "error", err)
		}

		var genErr error
		refreshToken, genErr = auth.GenerateRefreshToken()
		if genErr != nil {
			return fmt.Errorf("generate refresh token: %w", genErr)
		}

		refreshHash := auth.HashToken(refreshToken)
		session := &model.Session{
			UserId:      user.ID,
			RefreshHash: refreshHash,
			ExpiresAt:   time.Now().UTC().Add(s.cfg.RefreshTokenTTL),
			Revoked:     false,
		}

		if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		accessToken, genErr = auth.GenerateAccessToken(user.ID, s.cfg.AccessTokenTTL)
		if genErr != nil {
			return fmt.Errorf("generate access token: %w", genErr)
		}

		return nil
	}

	if s.txManager != nil {
		if err := s.txManager.WithTransaction(ctx, createSessionFn); err != nil {
			return nil, fmt.Errorf("finish login transaction: %w", err)
		}
	} else {
		if err := createSessionFn(ctx); err != nil {
			return nil, err
		}
	}

	totpEnabled := false
	if s.totpRepo != nil {
		totpEnabled, _ = s.totpRepo.IsTOTPEnabled(ctx, user.ID)
	}

	log.Info("passkey service: user successfully logged in with passkey", "user_id", user.ID, "passkey_id", passkey.ID)

	return &dto.AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		User: &dto.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			Name:          user.Name,
			TOTPEnabled:   totpEnabled,
			Gender:        user.Gender,
			Age:           user.Age,
			WeightKg:      user.WeightKg,
			HeightCm:      user.HeightCm,
			RestingHR:     user.RestingHR,
			MaxHR:         user.MaxHR,
			WeeklyRuns:    user.WeeklyRuns,
			ThresholdPace: user.ThresholdPace,
		},
	}, nil
}

// 5. GetUserPasskeys возвращает список привязанных ключей пользователя
func (s *passkeyService) GetUserPasskeys(ctx context.Context, userID int64) ([]model.Passkey, error) {
	return s.passkeyRepo.GetByUserID(ctx, userID)
}

// 6. DeletePasskey удаляет ключ (с проверкой прав доступа)
func (s *passkeyService) DeletePasskey(ctx context.Context, id int64, userID int64) error {
	return s.passkeyRepo.Delete(ctx, id, userID)
}
