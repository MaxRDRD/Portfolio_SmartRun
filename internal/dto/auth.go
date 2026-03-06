package dto

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=4"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
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
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	TOTPEnabled bool   `json:"totp_enabled"`
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
