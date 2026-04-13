package postgres

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/internal/repository_impl/postgres"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type workoutRepository struct {
	db repository.DB
}

func NewWorkoutRepository(db repository.DB) repository.WorkoutRepository {
	return &workoutRepository{db: db}
}

func (r *workoutRepository) getDB(ctx context.Context) repository.DB {
	if tx, ok := postgres.GetTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *workoutRepository) Create(ctx context.Context, workout *model.Workouts) error {
	db := r.getDB(ctx)

	sqlQuery := `
        INSERT INTO workouts (
            user_id, date, distance, duration, pace, type_activity,
            calories, avg_hr, max_hr, elevation_gain, avg_cadence, max_cadence,
            notes, shoes, vo2max_estimate, aerobic_training_effect,
			anaerobic_training_effect, training_load, training_stress_score,
			intensity_factor, avg_stress, sdrr_hrv, rmssd_hrv, time_in_hr_zone, recovery_time,
			rpe, efficiency, primary_training_focus, elevation_loss
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29
        )
        RETURNING id, created_at
    `
	err := db.QueryRow(ctx, sqlQuery,
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
		workout.TrainingStressScore,
		workout.IntensityFactor,
		workout.AvgStress,
		workout.SdrrHrv,
		workout.RmssdHrv,
		toInt32Slice(workout.TimeInHrZone),
		workout.RecoveryTime,
		workout.RPE,
		workout.Efficiency,
		workout.PrimaryTrainingFocus,
		workout.ElevationLoss,
	).Scan(&workout.ID, &workout.CreatedAt)
	if err != nil {
		return err
	}

	if workout.ID > 0 {
		return nil
	}

	// Safety fallback: in rare cases the caller may get zero ID despite successful insert.
	fallbackQuery := `
		SELECT id, created_at
		FROM workouts
		WHERE user_id = $1
		  AND date = $2
		  AND distance = $3
		  AND duration = $4
		  AND type_activity = $5
		ORDER BY id DESC
		LIMIT 1
	`

	if fbErr := db.QueryRow(ctx, fallbackQuery,
		workout.UserID,
		workout.Date,
		workout.Distance,
		workout.Duration,
		workout.TypeActivity,
	).Scan(&workout.ID, &workout.CreatedAt); fbErr != nil {
		return fmt.Errorf("create workout fallback lookup: %w", fbErr)
	}

	if workout.ID <= 0 {
		return fmt.Errorf("create workout: persisted row has invalid id")
	}

	return nil
}

func (r *workoutRepository) GetByID(ctx context.Context, id int64, userID int64) (*model.Workouts, error) {
	db := r.getDB(ctx)

	sqlQuery := fmt.Sprintf(`
		SELECT %s
		FROM workouts 
		WHERE id = $1 AND user_id = $2
	`, selectWorkoutColumns)

	row := db.QueryRow(ctx, sqlQuery, id, userID)
	workout, err := r.scanWorkoutRow(row)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, my_errors.ErrWorkoutNotFound
		}
		return nil, fmt.Errorf("scan workout: %w", err)
	}

	workout.ID = id
	workout.UserID = userID
	return workout, nil
}

// GetAllByUserID получает все тренировки пользователя с опциональными фильтрами
func (r *workoutRepository) GetAllByUserID(ctx context.Context, filter dto.WorkoutFilter) ([]model.Workouts, error) {
	db := r.getDB(ctx)

	// Построение WHERE условий
	whereClause, args, argPos := r.buildWorkoutFilters(filter)

	// Построение ORDER BY/LIMIT/OFFSET
	orderAndPaginationClause, orderArgs := r.buildOrderAndPagination(filter, argPos)
	args = append(args, orderArgs...)

	// Финальный SQL
	query := fmt.Sprintf(`
		SELECT %s
		FROM workouts
		%s%s
	`, selectWorkoutColumns, whereClause, orderAndPaginationClause)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workouts: %w", err)
	}
	defer rows.Close()

	var workouts []model.Workouts
	for rows.Next() {
		workout, err := r.scanWorkoutRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workout: %w", err)
		}
		workouts = append(workouts, *workout)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return workouts, nil
}

func (r *workoutRepository) DeleteWorkout(ctx context.Context, id int64, userID int64) error {
	db := r.getDB(ctx)

	sqlQuery := `
        DELETE FROM workouts
		WHERE id = $1 AND user_id = $2;
    `
	tag, err := db.Exec(ctx, sqlQuery, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrWorkoutNotFound
	}

	return nil
}

func (r *workoutRepository) Update(ctx context.Context, workout *model.Workouts) error {
	db := r.getDB(ctx)

	sqlQuery := `
        UPDATE workouts SET
        distance = $1, duration = $2, pace = $3, type_activity = $4, date = $5,
        calories = $6, avg_hr = $7, max_hr = $8, elevation_gain = $9,
        avg_cadence = $10, max_cadence = $11, notes = $12, shoes = $13,
        aerobic_training_effect = $14, anaerobic_training_effect = $15,
		training_load = $16, training_stress_score = $17,
		intensity_factor = $18, avg_stress = $19, sdrr_hrv = $20, rmssd_hrv = $21,
		time_in_hr_zone = $22, recovery_time = $23, rpe = $24,
		efficiency = $25, primary_training_focus = $26,
		vo2max_estimate = $27, elevation_loss = $28
		WHERE id = $29 AND user_id = $30
    `
	tag, err := db.Exec(ctx, sqlQuery,
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
		workout.TrainingStressScore,
		workout.IntensityFactor,
		workout.AvgStress,
		workout.SdrrHrv,
		workout.RmssdHrv,
		toInt32Slice(workout.TimeInHrZone),
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
		return my_errors.ErrWorkoutUpdate
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrWorkoutNotFound
	}

	return nil
}

func (r *workoutRepository) GetMonthlyHistory(ctx context.Context, userID int64, monthsLimit, monthsOffset int) ([]model.WorkoutMonthHistory, error) {
	db := r.getDB(ctx)

	if monthsLimit <= 0 {
		monthsLimit = 3
	}
	if monthsOffset < 0 {
		monthsOffset = 0
	}

	monthQuery := `
		SELECT
			date_trunc('month', date)::date AS month_start,
			COUNT(*) AS workouts_count,
			COALESCE(SUM(distance), 0) AS total_distance,
			COALESCE(SUM(duration), 0) AS total_duration
		FROM workouts
		WHERE user_id = $1
		GROUP BY 1
		ORDER BY month_start DESC
		LIMIT $2 OFFSET $3
	`

	monthRows, err := db.Query(ctx, monthQuery, userID, monthsLimit, monthsOffset)
	if err != nil {
		return nil, my_errors.ErrQueryWorkoutHistory
	}
	defer monthRows.Close()

	months := make([]model.WorkoutMonthHistory, 0, monthsLimit)
	monthIndex := make(map[string]int, monthsLimit)
	monthArgs := make([]interface{}, 1, monthsLimit+1)
	monthArgs[0] = userID

	for monthRows.Next() {
		var month model.WorkoutMonthHistory
		if err := monthRows.Scan(&month.Month, &month.WorkoutsCount, &month.TotalDistance, &month.TotalDuration); err != nil {
			return nil, fmt.Errorf("scan month history summary: %w", err)
		}

		month.Workouts = make([]model.WorkoutPreview, 0)
		monthKey := month.Month.Format("2006-01-02")
		monthIndex[monthKey] = len(months)
		months = append(months, month)
		monthArgs = append(monthArgs, month.Month)
	}

	if err := monthRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate month history summary: %w", err)
	}

	if len(months) == 0 {
		return months, nil
	}

	workoutsQuery := `
		SELECT id, date, distance, duration, pace, type_activity, COALESCE(notes, '')
		FROM workouts
		WHERE user_id = $1 AND date_trunc('month', date)::date IN (`

	for i := 2; i <= len(monthArgs); i++ {
		if i > 2 {
			workoutsQuery += ", "
		}
		workoutsQuery += fmt.Sprintf("$%d", i)
	}

	workoutsQuery += `)
		ORDER BY date DESC, id DESC
	`

	workoutRows, err := db.Query(ctx, workoutsQuery, monthArgs...)
	if err != nil {
		return nil, my_errors.ErrQueryWorkoutHistory
	}
	defer workoutRows.Close()

	for workoutRows.Next() {
		var workout model.WorkoutPreview
		if err := workoutRows.Scan(
			&workout.ID,
			&workout.Date,
			&workout.Distance,
			&workout.Duration,
			&workout.Pace,
			&workout.TypeActivity,
			&workout.Place,
		); err != nil {
			return nil, fmt.Errorf("scan workout month preview: %w", err)
		}

		workout.PreviewImage = defaultWorkoutPreviewImage(workout.TypeActivity)

		monthStart := time.Date(workout.Date.Year(), workout.Date.Month(), 1, 0, 0, 0, 0, workout.Date.Location())
		monthKey := monthStart.Format("2006-01-02")

		idx, ok := monthIndex[monthKey]
		if !ok {
			continue
		}

		months[idx].Workouts = append(months[idx].Workouts, workout)
	}

	if err := workoutRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workout month preview: %w", err)
	}

	return months, nil
}
