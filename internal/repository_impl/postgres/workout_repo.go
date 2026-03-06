package postgres

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type workoutRepository struct {
	db repository.DB
}

func NewWorkoutRepository(db repository.DB) repository.WorkoutRepository {
	return &workoutRepository{db: db}
}

func (r *workoutRepository) Create(ctx context.Context, workout *model.Workouts) error {
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

func (r *workoutRepository) GetByID(ctx context.Context, id int, userID int) (*model.Workouts, error) {
	sqlQuery := `
	SELECT date, distance, duration, created_at, type_activity, pace
	FROM workouts
	WHERE id = $1 AND user_id = $2;
	`
	var workout model.Workouts
	err := r.db.QueryRow(ctx, sqlQuery, id, userID).Scan(
		&workout.Date,
		&workout.Distance,
		&workout.Duration,
		&workout.CreatedAt,
		&workout.TypeActivity,
		&workout.Pace,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, my_errors.ErrWorkoutNotFound
	}

	return &workout, err
}

func (r *workoutRepository) GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
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

	var workouts []model.Workouts

	for rows.Next() {
		var workout model.Workouts
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

func (r *workoutRepository) DeleteWorkout(ctx context.Context, id int, userID int) error {
	sqlQuery := `
        DELETE FROM workouts
		WHERE id = $1 AND user_id = $2;
    `
	tag, err := r.db.Exec(ctx, sqlQuery, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrWorkoutNotFound
	}

	return nil
}

func (r *workoutRepository) Update(ctx context.Context, workout *model.Workouts) error {
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
		return my_errors.ErrWorkoutNotFound
	}

	return nil
}
