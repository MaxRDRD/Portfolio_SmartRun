package dto

import "time"

type WorkoutsResponse struct {
	ID                      int64    `json:"id"`
	Date                    string   `json:"date"`
	TypeActivity            string   `json:"type_activity"`
	Distance                float64  `json:"distance"`
	Duration                int      `json:"duration"`
	Pace                    float64  `json:"pace"` // или string "4:19"
	AvgCadence              *int     `json:"avg_cadence"`
	ElevationGain           *float64 `json:"elevation_gain"`
	ElevationLoss           *float64 `json:"elevation_loss"`
	AvgHR                   *int     `json:"avg_hr"`
	MaxHR                   *int     `json:"max_hr"`
	Calories                *int     `json:"calories"`
	VO2MaxEstimate          *float64 `json:"vo2max_estimate,omitempty"`
	RecoveryTime            *int     `json:"recovery_time,omitempty"`
	TrainingLoad            *float64 `json:"training_load,omitempty"`
	TrainingStressScore     *float64 `json:"training_stress_score,omitempty"`
	IntensityFactor         *float64 `json:"intensity_factor,omitempty"`
	AvgStress               *int     `json:"avg_stress,omitempty"`
	SdrrHrv                 *int     `json:"sdrr_hrv,omitempty"`
	RmssdHrv                *int     `json:"rmssd_hrv,omitempty"`
	PerceivedEffort         *int     `json:"perceived_effort,omitempty"`
	Notes                   string   `json:"notes,omitempty"`
	Shoes                   string   `json:"shoes,omitempty"`
	PrimaryTrainingFocus    string   `json:"primary_training_focus"`
	AerobicTrainingEffect   *float64 `json:"aerobic_training_effect"`
	AnaerobicTrainingEffect *float64 `json:"anaerobic_training_effect"`
}

type CreateRequest struct {
	Date         string  `json:"date" validate:"required"`
	Distance     float64 `json:"distance" validate:"required"`
	Duration     int     `json:"duration" validate:"required"`
	TypeActivity string  `json:"type_activity"`

	Calories      *int     `json:"calories,omitempty"`
	AvgHR         *int     `json:"avg_hr,omitempty"`
	MaxHR         *int     `json:"max_hr,omitempty"`
	AvgCadence    *int     `json:"avg_cadence,omitempty"`
	MaxCadence    *int     `json:"max_cadence,omitempty"`
	ElevationGain *float64 `json:"elevation_gain,omitempty"`
	ElevationLoss *float64 `json:"elevation_loss,omitempty"`

	RPE   *int    `json:"rpe,omitempty"` // Perceived Effort 1–10
	Notes *string `json:"notes,omitempty"`
	Shoes *string `json:"shoes,omitempty"`

	HRZones *HRZonesRequest `json:"hr_zones,omitempty"`
}

type UpdateRequest struct {
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

	HRZones *HRZonesRequest `json:"hr_zones,omitempty"`
}

type WorkoutFilter struct {
	UserID int64      `json:"user_id" validate:"required"`
	Type   string     `json:"type,omitempty"`
	From   *time.Time `json:"from,omitempty"`
	To     *time.Time `json:"to,omitempty"`
	Limit  int        `json:"limit,omitempty"`
	Offset int        `json:"offset,omitempty"`
}

type HRZonesRequest struct {
	Zone1Seconds int `json:"zone_1_seconds"` // < 60% HRmax (восстановление)
	Zone2Seconds int `json:"zone_2_seconds"` // 60-70%
	Zone3Seconds int `json:"zone_3_seconds"` // 70-80%
	Zone4Seconds int `json:"zone_4_seconds"` // 80-90%
	Zone5Seconds int `json:"zone_5_seconds"` // 90-100%
}

type WorkoutHistoryPreviewResponse struct {
	ID           int64   `json:"id"`
	Date         string  `json:"date"`
	Distance     float64 `json:"distance"`
	Duration     int     `json:"duration"`
	Pace         float64 `json:"pace"`
	TypeActivity string  `json:"type_activity"`
	Place        string  `json:"place"`
	PreviewImage string  `json:"preview_image"`
}

type WorkoutMonthHistoryResponse struct {
	Month         string                          `json:"month"`
	WorkoutsCount int                             `json:"workouts_count"`
	TotalDistance float64                         `json:"total_distance"`
	TotalDuration int                             `json:"total_duration"`
	Workouts      []WorkoutHistoryPreviewResponse `json:"workouts"`
}

type WorkoutHistoryResponse struct {
	Months []WorkoutMonthHistoryResponse `json:"months"`
}
