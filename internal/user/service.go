package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Register(ctx context.Context, name, email, password string) (*User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

// Создаём глобальный экземпляр валидатора
var validate *validator.Validate

func init() {
	validate = validator.New()
}

func (u *User) ValidateUser() error {
	err := validate.Struct(u)
	if err != nil {
		// Обработка ошибок валидации
		fmt.Println("Validation failed:", err)
		return err
	}
	return nil
}

// Функция для хеширования пароля
func hashPassword(password string) (string, error) {
	// GenerateFromPassword генерирует соль и хеширует пароль
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Функция для проверки пароля
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *service) Register(ctx context.Context, name, email, password string) (*User, error) {
	// 1. Проверяем, существует ли email
	existing, _ := s.repo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("user already exists")
	}

	// 2. Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var user = &User{
		Name:     name,
		Email:    email,
		Password: string(hash),
	}
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
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *service) Login(ctx context.Context, email, password string) (string, error) {

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(password),
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
