package model

import "time"

type Workouts struct {
	ID             int64
	UserID         int64
	Date           time.Time
	Distance       float64
	Duration       int
	Pace           float64
	CreatedAt      time.Time
	TypeActivity   string
	Calories       *int
	AvgHR          *int
	MaxHR          *int
	AvgCadence     *int
	MaxCadence     *int
	ElevationGain  *float64
	ElevationLoss  *float64
	VO2MaxEstimate *float64

	AerobicTrainingEffect   *float64
	AnaerobicTrainingEffect *float64

	PrimaryTrainingFocus string // "Aerobic", "Threshold", "VO2max", "Anaerobic", "Anaerobic Power"

	TrainingLoad *float64
	TrainingStressScore *float64
	IntensityFactor *float64
	RecoveryTime *int
	RPE          *int // субъективная нагрузка 1–10
	AvgStress    *int
	SdrrHrv      *int
	RmssdHrv     *int
	TimeInHrZone []int
	Notes        string
	Shoes        string
	Efficiency   *float64
}

type Split struct {
	WorkoutID int64
	Distance  float64
	Duration  int
	Pace      float64
}

type HeartRate struct {
	WorkoutID int64
	AvgHR     int
	MaxHR     int
}

type WorkoutPreview struct {
	ID           int64
	Date         time.Time
	Distance     float64
	Duration     int
	Pace         float64
	TypeActivity string
	Place        string
	PreviewImage string
}

type WorkoutMonthHistory struct {
	Month         time.Time
	WorkoutsCount int
	TotalDistance float64
	TotalDuration int
	Workouts      []WorkoutPreview
}
