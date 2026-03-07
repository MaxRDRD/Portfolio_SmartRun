package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

type UserHandler struct {
	service service.AuthService
}

func NewUserHandler(service service.AuthService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	var res *dto.AuthResult

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	res, err := h.service.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, my_errors.ErrUserAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,                    // 30 дней
		HttpOnly: true,                                 // ← обязательно!
		Secure:   os.Getenv("APP_ENV") == "production", // только HTTPS если в prod - в dev false
		SameSite: http.SameSiteStrictMode,              // или Lax — зависит от нужд
		// Domain:   "example.com",           // если нужно
	})

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(dto.UserResponse{
		ID:    res.User.ID,
		Email: res.User.Email,
		Name:  res.User.Name,
	})
	json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		Require2FA:  false,
	})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	var res *dto.AuthResult

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
	}

	res, err := h.service.Login(r.Context(), req)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), res.User.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
	}

	// Устанавливаем HttpOnly cookie с refresh-токеном
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,                    // 30 дней
		HttpOnly: true,                                 // ← обязательно!
		Secure:   os.Getenv("APP_ENV") == "production", // только HTTPS если в prod - в dev false
		SameSite: http.SameSiteStrictMode,              // или Lax — зависит от нужд
		// Domain:   "example.com",           // если нужно
	})

	// Отдаём клиенту только access token
	resp := dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
	}

	if totpEnabled {
		resp.Require2FA = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userResp, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
	}

	json.NewEncoder(w).Encode(dto.UserResponse{
		ID:          userResp.ID,
		Email:       userResp.Email,
		Name:        userResp.Name,
		TOTPEnabled: totpEnabled,
	})
}

func (h *UserHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "refresh token not found", http.StatusUnauthorized)
		return
	}

	var res *dto.AuthResult

	res, err = h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		// Важно: при любой ошибке (протух, отозван, подделан) → удаляем cookie
		h.deleteRefreshCookie(w)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Успешно → ставим новый refresh (ротация — лучшая практика)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/",
		MaxAge:   res.ExpiresIn,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
	})
}

func (h *UserHandler) deleteRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // ← удаляет cookie
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Пытаемся достать refresh-токен
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		// Удаляем из базы (даже если токен уже недействителен — не страшно)
		_ = h.service.Logout(r.Context(), cookie.Value) // Передаём plaintext
	}

	// В любом случае сбрасываем cookie
	h.deleteRefreshCookie(w)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out successfully",
	})
}

func (h *UserHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email, err := h.service.GetEmailByID(ctx, userID)
	if err != nil {
		http.Error(w, "email not found in context", http.StatusInternalServerError)
		return
	}

	secret, qrBytes, err := h.service.EnableTOTP(ctx, userID, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := dto.Enable2FAResponse{
		QRBase64: base64.StdEncoding.EncodeToString(qrBytes),
		Secret:   secret, // ← только здесь отдаём, потом уже никогда
	}

	// Возвращаем QR как PNG-картинку
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UserHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "code required", http.StatusBadRequest)
		return
	}

	valid, err := h.service.VerifyTOTP(ctx, userID, req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, "invalid 2fa code", http.StatusUnauthorized)
		return
	}

	var res *dto.AuthResult

	res, err = h.service.IssueTokensAfter2FA(ctx, userID)
	if err != nil {
		http.Error(w, "failed to issue tokens", http.StatusInternalServerError)
		return
	}

	// Ротация refresh (лучшая практика)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteStrictMode,
	})

	json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		Message:     "2FA verified",
	})
}

// POST /auth/password/reset/request
func (h *UserHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	// decode + validate

	err := h.service.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "failed to send reset email", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "if email exists, reset link sent"})
}

// POST /auth/password/reset/confirm
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password" validate:"required,min=8"`
	}
	// decode + validate

	userID, err := h.service.ValidateResetToken(r.Context(), req.Token)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}

	err = h.service.PerformPasswordReset(r.Context(), userID, req.NewPassword, req.Token)
	if err != nil {
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "password reset successful"})
}
