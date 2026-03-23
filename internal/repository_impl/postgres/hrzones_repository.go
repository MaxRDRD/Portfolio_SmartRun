package postgres

import (
	"SmartRun/internal/model"
	"SmartRun/internal/repository"
	"SmartRun/pkg/my_errors"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type hrZonesRepo struct {
	db repository.DB
}

func NewHRZonesRepository(db repository.DB) repository.HRZonesRepository {
	return &hrZonesRepo{db: db}
}

func (r *hrZonesRepo) getDB(ctx context.Context) repository.DB {
	if tx, ok := getTx(ctx); ok {
		return tx
	}
	return r.db // pool
}

func (r *hrZonesRepo) Upsert(ctx context.Context, zones *model.WorkoutHRZones) error {
	db := r.getDB(ctx)

	sql := `
	INSERT INTO hrzones (workout_id, zone1, zone2, zone3, zone4, zone5)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (workout_id) DO UPDATE SET
		zone1 = EXCLUDED.zone1,
		zone2 = EXCLUDED.zone2,
		zone3 = EXCLUDED.zone3,
		zone4 = EXCLUDED.zone4,
		zone5 = EXCLUDED.zone5
	`

	tag, err := db.Exec(ctx, sql,
		zones.WorkoutID,
		zones.Zone1Seconds,
		zones.Zone2Seconds,
		zones.Zone3Seconds,
		zones.Zone4Seconds,
		zones.Zone5Seconds,
	)

	if err != nil {
		return fmt.Errorf("hrzones upsert: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrHRZonesNotFound
	}

	return nil
}

func (r *hrZonesRepo) GetByWorkoutID(ctx context.Context, workoutID int64) (*model.WorkoutHRZones, error) {
	db := r.getDB(ctx)

	sql := `
	SELECT
		zone1, zone2, zone3, zone4, zone5
	FROM hrzones
        WHERE workout_id = $1
	`

	var hrzones model.WorkoutHRZones
	err := db.QueryRow(ctx, sql, workoutID).Scan(
		&hrzones.Zone1Seconds,
		&hrzones.Zone2Seconds,
		&hrzones.Zone3Seconds,
		&hrzones.Zone4Seconds,
		&hrzones.Zone5Seconds,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, my_errors.ErrHRZonesNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("hrzones get: %w", err)
	}
	hrzones.WorkoutID = workoutID
	return &hrzones, nil
}

func (r *hrZonesRepo) DeleteByWorkoutID(ctx context.Context, workoutID int64) error {
	db := r.getDB(ctx)
	sqlQuery := `
        DELETE FROM hrzones
		WHERE workout_id = $1
    `
	tag, err := db.Exec(ctx, sqlQuery, workoutID)
	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return my_errors.ErrHRZonesNotFound
	}

	return nil
}
