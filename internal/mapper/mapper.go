package mapper

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
)

func ToWorkoutsResponse(w *model.Workouts) dto.WorkoutsResponse {
	return dto.WorkoutsResponse{
		ID:                      w.ID,
		Date:                    w.Date.Format("2006-01-02"), // или time.RFC3339
		TypeActivity:            w.TypeActivity,
		Distance:                w.Distance,
		Duration:                w.Duration,
		Pace:                    w.Pace,
		AvgCadence:              w.AvgCadence,
		ElevationGain:           w.ElevationGain,
		ElevationLoss:           w.ElevationLoss,
		AvgHR:                   w.AvgHR,
		MaxHR:                   w.MaxHR,
		Calories:                w.Calories,
		VO2MaxEstimate:          w.VO2MaxEstimate,
		RecoveryTime:            w.RecoveryTime,
		TrainingLoad:            w.TrainingLoad,
		PerceivedEffort:         w.RPE, // если поле RPE int в модели
		Notes:                   w.Notes,
		Shoes:                   w.Shoes,
		PrimaryTrainingFocus:    w.PrimaryTrainingFocus,
		AerobicTrainingEffect:   w.AerobicTrainingEffect,
		AnaerobicTrainingEffect: w.AnaerobicTrainingEffect,
	}
}
