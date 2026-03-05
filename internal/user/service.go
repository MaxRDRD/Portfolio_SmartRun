package user

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"time"

	"github.com/go-playground/validator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResult, error)
	Login(ctx context.Context, req LoginRequest) (*AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	GetByID(ctx context.Context, id int64) (*UserResponse, error)
	DeleteRefreshToken(ctx context.Context, refresh_hash string) error
	Refresh(ctx context.Context, refreshToken string) (*AuthResult, error)
	EnableTOTP(ctx context.Context, userID int64, email string) (string, []byte, error) // secret, QR bytes
	VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error)
	IsTOTPEnabled(ctx context.Context, userID int64) (bool, error)
	IssueTokensAfter2FA(ctx context.Context, userID int64) (*AuthResult, error)
}

type service struct {
	repo     Repository
	validate *validator.Validate
}

func NewService(repo Repository) Service {
	return &service{
		repo:     repo,
		validate: validator.New(),
	}
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (*AuthResult, error) {
	user := &User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	}
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	// 1. Проверяем, существует ли email
	_, err := s.repo.GetByEmail(ctx, user.Email)

	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	// 2. Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user.Password = string(hash)

	// 3. Сохраняем
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // безопасный откат, если не будет Commit

	err = s.repo.CreateTx(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshHash := sha512.New()
	refreshHash.Write([]byte(refreshToken))
	refreshHashStr := hex.EncodeToString(refreshHash.Sum(nil)) // или fmt.Sprintf("%x", sha512.Sum512([]byte(refreshToken)))

	session := &Session{
		UserId:      int64(user.ID),
		RefreshHash: refreshHashStr,
		ExpiresAt:   time.Now().UTC().Add(30 * 24 * time.Hour),
		Revoked:     false,
		// ID и CreatedAt — НЕ заполняем, БД сама сделает
	}

	err = s.repo.CreateSessionTx(ctx, tx, session)

	if errors.Is(err, ErrInvalidToken) {
		return nil, err
	}

	accessToken, err := GenerateAccessToken(user.ID, AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	userResp := &UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		TOTPEnabled: false,
	}

	res := &AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    int(AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		User:         userResp,
	}

	return res, nil
}

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 64) // 64 байта = 512 бит
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("ошибка генерации байтов: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateAccessTokenTyped — вариант с явной структурой claims
func GenerateAccessToken(userID int64, duration time.Duration) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not set")
	}

	claims := AccessTokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			// Subject:   fmt.Sprintf("%d", userID), // можно и здесь, если хочешь
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

/*
func GenerateToken(userID int, n time.Duration) (string, error) {
	// Создаем Claims (полезная нагрузка)
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * n).Unix(), // Токен истекает через 900 минут
		"iat":     time.Now().Unix(),
	}

	// Создаем токен с методом подписи и claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("SECRET_KEY")
	if secret == "" {
		return "", errors.New("SECRET_KEY environment variable is not set")
	}
	// Подписываем токен
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
*/

func (s *service) Login(ctx context.Context, req LoginRequest) (*AuthResult, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}

	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(req.Password),
	)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	tx, err := s.repo.BeginTx(ctx) // Начинаем транзакцию
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // Автоматический откат, если не commit

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshHash := sha512.New()
	refreshHash.Write([]byte(refreshToken))
	refreshHashStr := hex.EncodeToString(refreshHash.Sum(nil)) // или fmt.Sprintf("%x", sha512.Sum512([]byte(refreshToken)))

	err = s.repo.DeleteAllSessionsForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	session := &Session{
		UserId:      int64(user.ID),
		RefreshHash: refreshHashStr,
		ExpiresAt:   time.Now().UTC().Add(30 * 24 * time.Hour),
		Revoked:     false,
		// ID и CreatedAt — НЕ заполняем, БД сама сделает
	}

	err = s.repo.CreateSessionTx(ctx, tx, session)
	if err != nil {
		return nil, err
	}

	accessToken, err := GenerateAccessToken(user.ID, AccessTokenTTL)
	expiresIn := int(AccessTokenTTL.Seconds()) // 900

	userResp := &UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		TOTPEnabled: false,
	}

	res := &AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
		User:         userResp,
	}

	return res, nil

}

func (s *service) GetByID(ctx context.Context, id int64) (*UserResponse, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}, nil
}

func (s *service) DeleteRefreshToken(ctx context.Context, refresh_hash string) error {
	err := s.repo.DeleteSessionByHash(ctx, refresh_hash)
	return err
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil // уже разлогинены
	}
	hash := sha512Hex(refreshToken) // твоя функция
	return s.repo.DeleteSessionByHash(ctx, hash)
}

func sha512Hex(input string) string {
	hash := sha512.Sum512([]byte(input))
	return hex.EncodeToString(hash[:])
}

// Refresh обновляет access-токен и выполняет ротацию refresh-токена
func (s *service) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	if refreshToken == "" {
		return nil, ErrInvalidToken
	}

	// Вычисляем хеш входящего refresh-токена
	hash := sha512.Sum512([]byte(refreshToken))
	refreshHash := hex.EncodeToString(hash[:])

	// Ищем существующую сессию
	session, err := s.repo.FindSessionByHash(ctx, refreshHash)
	if err != nil {
		// Здесь уже может быть ErrInvalidToken или ErrTokenNotFound
		return nil, fmt.Errorf("find session: %w", err)
	}

	// Дополнительная проверка (на всякий случай, если FindSessionByHash её пропустил)
	if session.Revoked || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrInvalidToken
	}

	// Генерируем новый access token
	newAccessToken, err := GenerateAccessToken(session.UserId, AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Генерируем новый refresh token (ротация)
	newRefreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Вычисляем новый хеш
	newHashBytes := sha512.Sum512([]byte(newRefreshToken))
	newRefreshHash := hex.EncodeToString(newHashBytes[:])

	// === Ротация: два популярных подхода ===

	// Вариант А: Удаляем старую сессию + создаём новую (чистая БД, проще отловить replay-атаки)
	err = s.repo.DeleteSessionByHash(ctx, refreshHash)
	if err != nil {
		return nil, fmt.Errorf("delete old session: %w", err)
	}

	newSession := &Session{
		UserId:      session.UserId,
		RefreshHash: newRefreshHash,
		ExpiresAt:   time.Now().UTC().Add(RefreshTokenTTL),
		Revoked:     false,
	}

	err = s.repo.CreateSession(ctx, newSession)
	if err != nil {
		// Здесь можно логировать, но клиенту всё равно ошибка
		return nil, fmt.Errorf("create new session: %w", err)
	}

	// Вариант Б (альтернатива): просто обновляем существующую запись
	// err = s.repo.UpdateSessionHash(ctx, refreshHash, newRefreshHash, newExpiresAt)
	// — но требует дополнительного метода в repository

	res := &AuthResult{
		AccessToken:  newAccessToken,
		ExpiresIn:    int(RefreshTokenTTL),
		RefreshToken: newRefreshToken,
	}

	return res, nil
}

func (s *service) EnableTOTP(ctx context.Context, userID int64, email string) (string, []byte, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "SmartRun",
		AccountName: email,
	})
	if err != nil {
		return "", nil, err
	}

	// Сохраняем в БД
	err = s.repo.UpdateTOTPSecret(ctx, userID, key.Secret(), true)
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

func (s *service) VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error) {
	secret, err := s.repo.GetTOTPSecret(ctx, userID)
	if err != nil {
		return false, err
	}
	return totp.Validate(code, secret), nil
}

func (s *service) IsTOTPEnabled(ctx context.Context, userID int64) (bool, error) {
	TOTPEnabled, err := s.repo.IsTOTPEnabled(ctx, userID)
	if err != nil {
		return false, err
	}
	return TOTPEnabled, nil
}

func (s *service) IssueTokensAfter2FA(ctx context.Context, userID int64) (*AuthResult, error) {

	tx, err := s.repo.BeginTx(ctx) // Начинаем транзакцию
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // Автоматический откат, если не commit

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshHash := sha512.New()
	refreshHash.Write([]byte(refreshToken))
	refreshHashStr := hex.EncodeToString(refreshHash.Sum(nil)) // или fmt.Sprintf("%x", sha512.Sum512([]byte(refreshToken)))

	session := &Session{
		UserId:      userID,
		RefreshHash: refreshHashStr,
		ExpiresAt:   time.Now().UTC().Add(30 * 24 * time.Hour),
		Revoked:     false,
	}

	err = s.repo.CreateSessionTx(ctx, tx, session)
	if err != nil {
		return nil, err
	}

	accessToken, err := GenerateAccessToken(userID, AccessTokenTTL)
	expiresIn := int(AccessTokenTTL.Seconds()) // 900

	res := &AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
	}

	return res, nil
}
