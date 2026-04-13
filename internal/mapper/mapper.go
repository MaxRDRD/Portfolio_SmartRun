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
		TrainingStressScore:     w.TrainingStressScore,
		IntensityFactor:         w.IntensityFactor,
		AvgStress:               w.AvgStress,
		SdrrHrv:                 w.SdrrHrv,
		RmssdHrv:                w.RmssdHrv,
		PerceivedEffort:         w.RPE, // если поле RPE int в модели
		HRZones:                 toHRZonesResponse(w.TimeInHrZone),
		Notes:                   w.Notes,
		Shoes:                   w.Shoes,
		PrimaryTrainingFocus:    w.PrimaryTrainingFocus,
		AerobicTrainingEffect:   w.AerobicTrainingEffect,
		AnaerobicTrainingEffect: w.AnaerobicTrainingEffect,
	}
}

func toHRZonesResponse(zones []int) *dto.HRZonesRequest {
	if len(zones) == 0 {
		return nil
	}

	res := &dto.HRZonesRequest{}
	if len(zones) > 0 {
		res.Zone1Seconds = zones[0]
	}
	if len(zones) > 1 {
		res.Zone2Seconds = zones[1]
	}
	if len(zones) > 2 {
		res.Zone3Seconds = zones[2]
	}
	if len(zones) > 3 {
		res.Zone4Seconds = zones[3]
	}
	if len(zones) > 4 {
		res.Zone5Seconds = zones[4]
	}
	if len(zones) > 5 {
		res.Zone6Seconds = zones[5]
	}

	return res
}

func ToWorkoutHistoryResponse(months []model.WorkoutMonthHistory) dto.WorkoutHistoryResponse {
	response := dto.WorkoutHistoryResponse{
		Months: make([]dto.WorkoutMonthHistoryResponse, len(months)),
	}

	for i, month := range months {
		previews := make([]dto.WorkoutHistoryPreviewResponse, len(month.Workouts))
		for j, workout := range month.Workouts {
			previews[j] = dto.WorkoutHistoryPreviewResponse{
				ID:           workout.ID,
				Date:         workout.Date.Format("2006-01-02"),
				Distance:     workout.Distance,
				Duration:     workout.Duration,
				Pace:         workout.Pace,
				TypeActivity: workout.TypeActivity,
				Place:        workout.Place,
				PreviewImage: workout.PreviewImage,
			}
		}

		response.Months[i] = dto.WorkoutMonthHistoryResponse{
			Month:         month.Month.Format("2006-01"),
			WorkoutsCount: month.WorkoutsCount,
			TotalDistance: month.TotalDistance,
			TotalDuration: month.TotalDuration,
			Workouts:      previews,
		}
	}

	return response
}
