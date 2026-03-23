package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type userRepository struct {
	db repository.DB
}

func NewUserRepository(db repository.DB) repository.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := getTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *userRepository) CreateUser(ctx context.Context, user *model.User) error {
	db := r.getDB(ctx)

	query := `
        INSERT INTO users (
			name, email, password,
			gender, age, weight_kg, height_cm,
			resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
			created_at
		)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
        RETURNING id, created_at
    `
	return db.QueryRow(ctx, query,
		user.Name,
		user.Email,
		user.Password,
		user.Gender,
		user.Age,
		user.WeightKg,
		user.HeightCm,
		user.RestingHR,
		user.MaxHR,
		user.WeeklyRuns,
		user.ThresholdPace,
	).Scan(&user.ID, &user.CreatedAt)
}

/*
	ID        int       `json:"id" validate:"required,min=2,max=50"`
	Name      string    `json:"name" validate:"required,min=4"`
	Email     string    `json:"email" validate:"required,email"`
	Password  string    `json:"pass" validate:"required,min=8"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
*/

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	db := r.getDB(ctx)

	query := `
	SELECT id, name, email, password,
	       gender, age, weight_kg, height_cm,
	       resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
	       created_at
	FROM users
	WHERE email = $1;
	`

	var user model.User
	err := db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Gender,
		&user.Age,
		&user.WeightKg,
		&user.HeightCm,
		&user.RestingHR,
		&user.MaxHR,
		&user.WeeklyRuns,
		&user.ThresholdPace,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, my_errors.ErrUserNotFound
	}

	return &user, err
}

func (r *userRepository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	db := r.getDB(ctx)

	query := `
	SELECT id, name, email, password,
	       gender, age, weight_kg, height_cm,
	       resting_hr, max_hr, weekly_runs, threshold_pace_min_km,
	       created_at
	FROM users
	WHERE id = $1;
	`

	var user model.User
	err := db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Gender,
		&user.Age,
		&user.WeightKg,
		&user.HeightCm,
		&user.RestingHR,
		&user.MaxHR,
		&user.WeeklyRuns,
		&user.ThresholdPace,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, my_errors.ErrUserNotFound
	}

	return &user, err
}

func (r *userRepository) GetEmailByID(ctx context.Context, id int64) (string, error) {
	db := r.getDB(ctx)

	query := `
	SELECT email FROM users WHERE id = $1;
	`

	var email string
	err := db.QueryRow(ctx, query, id).Scan(
		&email,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", my_errors.ErrUserNotFound
	}

	return email, err
}


func (r *userRepository) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
	db := r.getDB(ctx)

	query := `
	UPDATE users
	SET password = $1
	WHERE id = $2
	`

	tag, err := db.Exec(ctx, query, newHash, userID)

	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return my_errors.ErrUserNotFound
	}
	return nil
}
