package dto

type UpdateUserRequest struct {
	Name          *string  `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Email         *string  `json:"email,omitempty" validate:"omitempty,email"`
	Gender        *string  `json:"gender,omitempty" validate:"omitempty,oneof=male female other"`
	Age           *int     `json:"age,omitempty" validate:"omitempty,min=10,max=120"`
	WeightKg      *float64 `json:"weight_kg,omitempty" validate:"omitempty,min=25,max=350"`
	HeightCm      *float64 `json:"height_cm,omitempty" validate:"omitempty,min=100,max=250"`
	RestingHR     *int     `json:"resting_hr,omitempty" validate:"omitempty,min=30,max=120"`
	MaxHR         *int     `json:"max_hr,omitempty" validate:"omitempty,min=120,max=240"`
	WeeklyRuns    *int     `json:"weekly_runs,omitempty" validate:"omitempty,min=0,max=14"`
	ThresholdPace *float64 `json:"threshold_pace_min_km,omitempty" validate:"omitempty,min=2,max=12"`
}
