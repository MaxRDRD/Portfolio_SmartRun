package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int) (*User, error)
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, user *User) error {
	query := `
        INSERT INTO users (name, email, password, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING id, created_at
    `
	return r.db.QueryRow(ctx, query,
		user.Name,
		user.Email,
		user.Password,
	).Scan(&user.ID, &user.CreatedAt)
}

/*
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Name      string    `json:"name" validate:"required,min=4"`
	Email     string    `json:"email" validate:"required,email"`
	Password  string    `json:"pass" validate:"required,min=8"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
*/

func (r *repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
	SELECT * FROM users WHERE email = $1;
	`

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Password,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)

	}

	return &user, err
}

func (r *repository) GetByID(ctx context.Context, id int) (*User, error) {
	query := `
	SELECT * FROM users WHERE id = $1;
	`

	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.Name,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)

	}

	return &user, err
}
