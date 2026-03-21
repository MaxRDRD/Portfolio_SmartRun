package validate

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"time"
)

/*
	Date         string  `json:"date"`
    Distance     float64 `json:"distance"`
    Duration     int     `json:"duration"`
    TypeActivity string  `json:"type_activity"`

    Calories      *int     `json:"calories,omitempty"`
    AvgHR         *int     `json:"avg_hr,omitempty"`
    MaxHR         *int     `json:"max_hr,omitempty"`
    AvgCadence    *int     `json:"avg_cadence,omitempty"`
    MaxCadence    *int     `json:"max_cadence,omitempty"`
    ElevationGain *float64 `json:"elevation_gain,omitempty"`
    ElevationLoss *float64 `json:"elevation_loss,omitempty"`
    RPE           *int     `json:"rpe,omitempty"`
    Notes         *string  `json:"notes,omitempty"`
    Shoes         *string  `json:"shoes,omitempty"`
*/

func ApplyUpdateRequest(w *model.Workouts, req dto.UpdateRequest) {
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err == nil { // игнорируем ошибку или возвращаем её — решай сам
			w.Date = parsed
		}
	}

	if req.Distance > 0 {
		w.Distance = req.Distance
	}

	if req.Duration > 0 {
		w.Duration = req.Duration
	}

	if req.TypeActivity != "" {
		w.TypeActivity = req.TypeActivity
	}

	if req.Calories != nil {
		w.Calories = req.Calories
	}

	if req.AvgHR != nil {
		w.AvgHR = req.AvgHR
	}

	if req.MaxHR != nil {
		w.MaxHR = req.MaxHR
	}

	if req.AvgCadence != nil {
		w.AvgCadence = req.AvgCadence
	}

	if req.MaxCadence != nil {
		w.MaxCadence = req.MaxCadence
	}

	if req.ElevationGain != nil {
		w.ElevationGain = req.ElevationGain
	}

	if req.ElevationLoss != nil {
		w.ElevationLoss = req.ElevationLoss
	}

	if req.RPE != nil {
		w.RPE = req.RPE
	}

	if req.Notes != nil {
		w.Notes = *req.Notes
	}

	if req.Shoes != nil {
		w.Shoes = *req.Shoes
	}
}
