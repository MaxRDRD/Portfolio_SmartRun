package user

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type User struct {
	ID       int    `json:"id" validate:"required,min=2,max=50"`
	Name     string `json:"name" validate:"required,min=4"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"pass" validate:"required,min=8"`
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

// DTO
type RegisterRequest struct {
}

type LoginRequest struct {
}
