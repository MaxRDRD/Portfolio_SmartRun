package user

import (
	"time"
)

type User struct {
	ID          int64
	Name        string
	Email       string
	Password    string
	CreatedAt   time.Time
	TOTPEnabled bool //Флаг, включена ли 2FA.
}
