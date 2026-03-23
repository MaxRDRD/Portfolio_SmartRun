package dto

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=4"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`

	// Optional profile fields for better running analytics.
	Gender        string   `json:"gender,omitempty" validate:"omitempty,oneof=male female other"`
	Age           *int     `json:"age,omitempty" validate:"omitempty,min=10,max=120"`
	WeightKg      *float64 `json:"weight_kg,omitempty" validate:"omitempty,min=25,max=350"`
	HeightCm      *float64 `json:"height_cm,omitempty" validate:"omitempty,min=100,max=250"`
	RestingHR     *int     `json:"resting_hr,omitempty" validate:"omitempty,min=30,max=120"`
	MaxHR         *int     `json:"max_hr,omitempty" validate:"omitempty,min=120,max=240"`
	WeeklyRuns    *int     `json:"weekly_runs,omitempty" validate:"omitempty,min=0,max=14"`
	ThresholdPace *float64 `json:"threshold_pace_min_km,omitempty" validate:"omitempty,min=2,max=12"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_access_token"`
	Require2FA  bool   `json:"require_2fa,omitempty"`
	Message     string `json:"message"`
}

type UserResponse struct {
	ID            int64   `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	TOTPEnabled   bool    `json:"totp_enabled"`
	Gender        string  `json:"gender,omitempty"`
	Age           int     `json:"age,omitempty"`
	WeightKg      float64 `json:"weight_kg,omitempty"`
	HeightCm      float64 `json:"height_cm,omitempty"`
	RestingHR     int     `json:"resting_hr,omitempty"`
	MaxHR         int     `json:"max_hr,omitempty"`
	WeeklyRuns    int     `json:"weekly_runs,omitempty"`
	ThresholdPace float64 `json:"threshold_pace_min_km,omitempty"`
}

type Enable2FAResponse struct {
	QRBase64 string `json:"qr_base64,omitempty"`
	Secret   string `json:"secret,omitempty"`
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	User         *UserResponse
}
