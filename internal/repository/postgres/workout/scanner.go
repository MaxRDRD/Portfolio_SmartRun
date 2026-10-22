package postgres

import (
	"SmartRun/internal/model"
	"fmt"
)

// scanner задает минимальный контракт для pgx.Row и pgx.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanWorkoutRow преобразует результат SQL в модель Workouts.
func (r *workoutRepository) scanWorkoutRow(row scanner) (*model.Workouts, error) {
	var workout model.Workouts
	var timeInHrZone []int32

	err := row.Scan(
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
		&workout.TrainingStressScore,
		&workout.IntensityFactor,
		&workout.AvgStress,
		&workout.SdrrHrv,
		&workout.RmssdHrv,
		&timeInHrZone,
		&workout.RecoveryTime,
		&workout.RPE,
		&workout.Efficiency,
		&workout.PrimaryTrainingFocus,
		&workout.ElevationLoss,
		&workout.IsAnomalous,
		&workout.AnomalyReason,
	)
	if err != nil {
		return nil, fmt.Errorf("scanWorkoutRow: %w", err)
	}

	workout.TimeInHrZone = toIntSlice(timeInHrZone)
	return &workout, nil
}
