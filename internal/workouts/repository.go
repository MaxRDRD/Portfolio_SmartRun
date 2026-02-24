package workouts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, workour *Workouts) error
	GetByID(ctx context.Context, id int, userID int) (*Workouts, error)
	GetAllByUserID(ctx context.Context, filter WorkoutFilter) ([]Workouts, error)
	DeleteWorkout(ctx context.Context, id int, userID int) error
	Update(ctx context.Context, workout *Workouts) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, workout *Workouts) error {
	sqlQuery := `
        INSERT INTO workouts (type_activity, user_id, distance, date, duration, pace)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, created_at
    `
	return r.db.QueryRow(ctx, sqlQuery,
		workout.TypeActivity,
		workout.UserID,
		workout.Distance,
		workout.Date,
		workout.Duration,
		workout.Pace,
	).Scan(&workout.ID, &workout.CreatedAt)
}

func (r *repository) GetByID(ctx context.Context, id int, userID int) (*Workouts, error) {
	sqlQuery := `
	SELECT date, distance, duration, created_at, type_activity, pace
	FROM workouts
	WHERE id = $1 AND user_id = $2;
	`
	var workout Workouts
	err := r.db.QueryRow(ctx, sqlQuery, id, userID).Scan(
		&workout.Date,
		&workout.Distance,
		&workout.Duration,
		&workout.CreatedAt,
		&workout.TypeActivity,
		&workout.Pace,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkoutNotFound
	}

	return &workout, err
}

func (r *repository) GetAllByUserID(ctx context.Context, filter WorkoutFilter) ([]Workouts, error) {
	query := `
		SELECT id, date, distance, duration, created_at, type_activity, pace
		FROM workouts
		WHERE user_id = $1
	`

	args := []interface{}{filter.UserID}
	argPos := 2

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type_activity = $%d", argPos)
		args = append(args, filter.Type)
		argPos++
	}

	if filter.From != nil {
		query += fmt.Sprintf(" AND date >= $%d", argPos)
		args = append(args, *filter.From)
		argPos++
	}

	if filter.To != nil {
		query += fmt.Sprintf(" AND date <= $%d", argPos)
		args = append(args, *filter.To)
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY date DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query all workouts: %w", err)
	}
	defer rows.Close()

	var workouts []Workouts

	for rows.Next() {
		var workout Workouts
		err := rows.Scan(
			&workout.ID,
			&workout.Date,
			&workout.Distance,
			&workout.Duration,
			&workout.CreatedAt,
			&workout.TypeActivity,
			&workout.Pace,
		)
		if err != nil {
			return nil, fmt.Errorf("scan workout: %w", err)
		}
		workouts = append(workouts, workout)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return workouts, nil
}

func (r *repository) DeleteWorkout(ctx context.Context, id int, userID int) error {
	sqlQuery := `
        DELETE FROM workouts
		WHERE id = $1 AND user_id = $2;
    `
	tag, err := r.db.Exec(ctx, sqlQuery, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrWorkoutNotFound
	}

	return nil
}

func (r *repository) Update(ctx context.Context, workout *Workouts) error {
	sqlQuery := `
        UPDATE workouts
		SET distance=$1, duration=$2, type_activity=$3, date=$4
		WHERE id=$5 AND user_id=$6
    `
	tag, err := r.db.Exec(ctx, sqlQuery,
		workout.Distance,
		workout.Duration,
		workout.TypeActivity,
		workout.Date,
		workout.ID,
		workout.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update workout: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrWorkoutNotFound
	}

	return nil
}
