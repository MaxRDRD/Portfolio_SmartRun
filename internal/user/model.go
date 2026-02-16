package user

import (
	"time"
)

type User struct {
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Name      string    `json:"name" validate:"required,min=4"`
	Email     string    `json:"email" validate:"required,email"`
	Password  string    `json:"pass" validate:"required,min=8,max=72"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
}

// DTO
type RegisterRequest struct {
}

type LoginRequest struct {
}
