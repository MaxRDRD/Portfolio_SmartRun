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

func (r *repository) Register(ctx context.Context, name, email, password string) (*User, error) {
	sqlQuery := `
	SELECT * FROM users WHERE email = $2;
	`
	err := r.db.QueryRow(ctx, sqlQuery, email)
	if err != nil {
		hashPasw, hashErr := hashPassword(password)
		if hashErr != nil {
			fmt.Errorf("failed to hash password: %w", err)
			return nil, hashErr
		}
		var user = User{
			Name:     name,
			Email:    email,
			Password: hashPasw,
		}
		createErr := r.Create(ctx, &user)
		if createErr != nil {
			fmt.Errorf("failed to create user: %w", err)
			return nil, createErr
		}

		return &user, nil

	}

	return nil, fmt.Errorf("database error: %w", err)

}

func GenerateToken(userID string) (string, error) {
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

func (r *repository) Login(ctx context.Context, email, password string) (string, error) {
	sqlQuery := `
	SELECT * FROM users WHERE email = $2;
	`

	err := r.db.QueryRow(ctx, sqlQuery, email)
	if err != nil {
		hash, hashErr := hashPassword(password)
		if hashErr != nil {
			fmt.Errorf("failed to hash password: %w", err)
			return "", hashErr
		}
		truePasw := checkPasswordHash(password, hash)
		if truePasw != true {
			fmt.Errorf("failed to hash password: %w", err)
			return "", fmt.Errorf("user not found")
		}

		token, tokenErr := GenerateToken(password)

		if tokenErr != nil {
			return "", errors.New("impossible to generate a token")
		}

		return token, nil

	}

	return "", fmt.Errorf("database error: %w", err)

}
