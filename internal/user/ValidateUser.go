package user

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

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
