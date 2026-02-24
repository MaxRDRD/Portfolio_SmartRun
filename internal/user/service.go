package user

import (
	"context"
	"errors"

	"os"
	"time"

	"github.com/go-playground/validator"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*User, error)
	Login(ctx context.Context, req LoginRequest) (string, error)
	GetByID(ctx context.Context, id int) (*User, error)
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

func (s *service) Register(ctx context.Context, req RegisterRequest) (*User, error) {
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
	err = s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func GenerateToken(userID int) (string, error) {
	// Создаем Claims (полезная нагрузка)
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 1).Unix(), // Токен истекает через 1 час
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

func (s *service) Login(ctx context.Context, req LoginRequest) (string, error) {
	if err := s.validate.Struct(req); err != nil {
		return "", err
	}

	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(req.Password),
	)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	token, err := GenerateToken(user.ID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *service) GetByID(ctx context.Context, id int) (*User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}
