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
        INSERT INTO workouts (
            user_id, date, distance, duration, pace, type_activity,
            calories, avg_hr, max_hr, elevation_gain, avg_cadence, max_cadence,
            notes, shoes, vo2max_estimate, aerobic_training_effect,
            anaerobic_training_effect, training_load, recovery_time,
            rpe, efficiency, primary_training_focus, elevation_loss
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
            $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
        )
        RETURNING id, created_at
    `
	return r.db.QueryRow(ctx, sqlQuery,
		workout.UserID,
		workout.Date,
		workout.Distance,
		workout.Duration,
		workout.Pace,
		workout.TypeActivity,
		workout.Calories,
		workout.AvgHR,
		workout.MaxHR,
		workout.ElevationGain,
		workout.AvgCadence,
		workout.MaxCadence,
		workout.Notes,
		workout.Shoes,
		workout.VO2MaxEstimate,
		workout.AerobicTrainingEffect,
		workout.AnaerobicTrainingEffect,
		workout.TrainingLoad,
		workout.RecoveryTime,
		workout.RPE,
		workout.Efficiency,
		workout.PrimaryTrainingFocus,
		workout.ElevationLoss,
	).Scan(&workout.ID, &workout.CreatedAt)
}

func (r *workoutRepository) GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error) {
	sqlQuery := `
	SELECT date, distance, duration, pace, type_activity, calories,
	       avg_hr, max_hr, elevation_gain, avg_cadence, max_cadence,
	       notes, shoes, vo2max_estimate, aerobic_training_effect, anaerobic_training_effect,
	       training_load, recovery_time, rpe, efficiency, primary_training_focus, elevation_loss
	FROM workouts WHERE id = $1 AND user_id = $2
	`
	var workout model.Workouts
	err := r.db.QueryRow(ctx, sqlQuery, id, userID).Scan(
		&workout.Date,
		&workout.Distance,
		&workout.Duration,
		&workout.Pace,
		&workout.TypeActivity,
		&workout.Calories,
		&workout.AvgHR,
		&workout.MaxHR,
		&workout.ElevationGain,
		&workout.AvgCadence,
		&workout.MaxCadence,
		&workout.Notes,
		&workout.Shoes,
		&workout.VO2MaxEstimate,
		&workout.AerobicTrainingEffect,
		&workout.AnaerobicTrainingEffect,
		&workout.TrainingLoad,
		&workout.RecoveryTime,
		&workout.RPE,
		&workout.Efficiency,
		&workout.PrimaryTrainingFocus,
		&workout.ElevationLoss,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, my_errors.ErrWorkoutNotFound
	}
	workout.ID = id
	workout.UserID = userID

	return &workout, err
}

func (r *workoutRepository) GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	query := `
		SELECT id, date, distance, duration, pace, type_activity, calories,
		       avg_hr, max_hr, elevation_gain, avg_cadence, max_cadence,
		       notes, shoes, vo2max_estimate, aerobic_training_effect, anaerobic_training_effect,
		       training_load, recovery_time, rpe, efficiency, primary_training_focus, elevation_loss
		FROM workouts WHERE user_id = $1
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

	limit, offset := filter.Limit, filter.Offset
	if limit <= 0 {
		limit = 100
	}

	query += fmt.Sprintf(" ORDER BY date DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

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
			&workout.Pace,
			&workout.TypeActivity,
			&workout.Calories,
			&workout.AvgHR,
			&workout.MaxHR,
			&workout.ElevationGain,
			&workout.AvgCadence,
			&workout.MaxCadence,
			&workout.Notes,
			&workout.Shoes,
			&workout.VO2MaxEstimate,
			&workout.AerobicTrainingEffect,
			&workout.AnaerobicTrainingEffect,
			&workout.TrainingLoad,
			&workout.RecoveryTime,
			&workout.RPE,
			&workout.Efficiency,
			&workout.PrimaryTrainingFocus,
			&workout.ElevationLoss,
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

func (r *workoutRepository) DeleteWorkout(ctx context.Context, id int64, userID int64) error {
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
        UPDATE workouts SET
        distance = $1, duration = $2, pace = $3, type_activity = $4, date = $5,
        calories = $6, avg_hr = $7, max_hr = $8, elevation_gain = $9,
        avg_cadence = $10, max_cadence = $11, notes = $12, shoes = $13,
        aerobic_training_effect = $14, anaerobic_training_effect = $15,
        training_load = $16, recovery_time = $17, rpe = $18,
        efficiency = $19, primary_training_focus = $20,
        vo2max_estimate = $21, elevation_loss = $22
        WHERE id = $23 AND user_id = $24
    `
	tag, err := r.db.Exec(ctx, sqlQuery,
		workout.Distance,
		workout.Duration,
		workout.Pace,
		workout.TypeActivity,
		workout.Date,
		workout.Calories,
		workout.AvgHR,
		workout.MaxHR,
		workout.ElevationGain,
		workout.AvgCadence,
		workout.MaxCadence,
		workout.Notes,
		workout.Shoes,
		workout.AerobicTrainingEffect,
		workout.AnaerobicTrainingEffect,
		workout.TrainingLoad,
		workout.RecoveryTime,
		workout.RPE,
		workout.Efficiency,
		workout.PrimaryTrainingFocus,
		workout.VO2MaxEstimate,
		workout.ElevationLoss,
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
