package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"context"
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
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req dto.RegisterRequest
	var res *dto.AuthResult

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("register: invalid body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	res, err := h.service.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, my_errors.ErrUserAlreadyExists) {
			log.Warn("register: user already exists", "email", req.Email)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		log.Error("register: service failed", "error", err, "email", req.Email)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("register: success", "user_id", res.User.ID, "email", res.User.Email)

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	type registerResponse struct {
		User dto.UserResponse `json:"user"`
		Auth dto.AuthResponse `json:"auth"`
	}

	_ = json.NewEncoder(w).Encode(registerResponse{
		User: *res.User,
		Auth: dto.AuthResponse{
			AccessToken: res.AccessToken,
			ExpiresIn:   res.ExpiresIn,
			Require2FA:  false,
			Message:     "registered",
		},
	})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req dto.LoginRequest
	var res *dto.AuthResult

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("login: invalid request", "error", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	res, err := h.service.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, my_errors.ErrInvalidCredentials) {
			log.Warn("login: invalid credentials", "email", req.Email)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		log.Error("login: internal error", "error", err, "email", req.Email)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), res.User.ID)
	if err != nil {
		log.Error("login: failed to check 2fa status", "error", err, "user_id", res.User.ID)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
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

	// Если TOTP включена: НЕ выдаём access token, требуем проверку 2FA
	if totpEnabled {
		log.Info("login: 2fa required", "user_id", res.User.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202 Accepted - требуется дополнительное действие
		_ = json.NewEncoder(w).Encode(dto.AuthResponse{
			AccessToken: "",              // пусто!
			ExpiresIn:   0,               // пусто!
			Require2FA:  true,
			Message:     "2FA required - call /verify-2fa with TOTP code",
		})
		return
	}

	// TOTP отключена - выдаём полный access token
	w.Header().Set("Content-Type", "application/json")
	log.Info("login: success", "user_id", res.User.ID)
	_ = json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		Message:     "ok",
	})
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("me: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userResp, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		log.Warn("me: user not found", "user_id", userID, "error", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), userID)
	if err != nil {
		log.Error("me: failed to check 2fa status", "user_id", userID, "error", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(dto.UserResponse{
		ID:            userResp.ID,
		Email:         userResp.Email,
		Name:          userResp.Name,
		TOTPEnabled:   totpEnabled,
		Gender:        userResp.Gender,
		Age:           userResp.Age,
		WeightKg:      userResp.WeightKg,
		HeightCm:      userResp.HeightCm,
		RestingHR:     userResp.RestingHR,
		MaxHR:         userResp.MaxHR,
		WeeklyRuns:    userResp.WeeklyRuns,
		ThresholdPace: userResp.ThresholdPace,
	})
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("update-me: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("update-me: invalid body", "error", err, "user_id", userID)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	updated, err := h.service.UpdateUser(ctx, userID, req)
	if err != nil {
		if errors.Is(err, my_errors.ErrUserAlreadyExists) {
			log.Warn("update-me: email already exists", "user_id", userID)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		log.Error("update-me: service failed", "error", err, "user_id", userID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	totpEnabled, err := h.service.IsTOTPEnabled(ctx, userID)
	if err != nil {
		log.Error("update-me: failed to check 2fa status", "user_id", userID, "error", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto.UserResponse{
		ID:            updated.ID,
		Email:         updated.Email,
		Name:          updated.Name,
		TOTPEnabled:   totpEnabled,
		Gender:        updated.Gender,
		Age:           updated.Age,
		WeightKg:      updated.WeightKg,
		HeightCm:      updated.HeightCm,
		RestingHR:     updated.RestingHR,
		MaxHR:         updated.MaxHR,
		WeeklyRuns:    updated.WeeklyRuns,
		ThresholdPace: updated.ThresholdPace,
	})
}

func (h *UserHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		log.Warn("refresh: cookie not found", "error", err)
		http.Error(w, "refresh token not found", http.StatusUnauthorized)
		return
	}

	var res *dto.AuthResult

	res, err = h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		// Важно: при любой ошибке (протух, отозван, подделан) → удаляем cookie
		log.Warn("refresh: invalid refresh token", "error", err)
		h.deleteRefreshCookie(ctx, w)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Успешно → ставим новый refresh
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	log.Info("refresh: success")
	_ = json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		Message:     "ok",
	})
}

func (h *UserHandler) deleteRefreshCookie(ctx context.Context, w http.ResponseWriter) {
	log := logger.FromContext(ctx)
	log.Debug("auth: deleting refresh cookie")

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
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Пытаемся достать refresh-токен
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		// Удаляем из базы (даже если токен уже недействителен — не страшно)
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			log.Warn("logout: failed to remove session", "error", err)
		}
	}

	// В любом случае сбрасываем cookie
	h.deleteRefreshCookie(ctx, w)
	log.Info("logout: success")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out successfully",
	})
}

func (h *UserHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("enable2fa: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email, err := h.service.GetEmailByID(ctx, userID)
	if err != nil {
		log.Error("enable2fa: failed to resolve email", "user_id", userID, "error", err)
		http.Error(w, "email not found in context", http.StatusInternalServerError)
		return
	}

	secret, qrBytes, err := h.service.EnableTOTP(ctx, userID, email)
	if err != nil {
		log.Error("enable2fa: failed to enable", "user_id", userID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Info("enable2fa: success", "user_id", userID)

	resp := dto.Enable2FAResponse{
		QRBase64: base64.StdEncoding.EncodeToString(qrBytes),
		Secret:   secret, // ← только здесь отдаём, потом уже никогда
	}

	// Возвращаем QR как PNG-картинку
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *UserHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Требуем refresh token из cookie для идентификации (БЕЗ JWT requirement)
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		log.Warn("verify2fa: refresh cookie required", "error", err)
		http.Error(w, "refresh_token cookie required", http.StatusUnauthorized)
		return
	}

	// Валидируем refresh token и получаем userID из сессии
	refreshHash := auth.HashToken(cookie.Value)
	session, err := h.service.GetSessionByHash(ctx, refreshHash) // нужно добавить этот метод
	if err != nil {
		log.Info("invalid or expired session")
		http.Error(w, "invalid or expired session", http.StatusUnauthorized)
		return
	}
	userID := session.UserId

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("verify2fa: invalid json", "error", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		log.Warn("verify2fa: empty code", "user_id", userID)
		http.Error(w, "code required", http.StatusBadRequest)
		return
	}

	// Проверяем TOTP code
	valid, err := h.service.VerifyTOTP(ctx, userID, req.Code)
	if err != nil {
		log.Error("verify2fa: totp verification failed", "user_id", userID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !valid {
		log.Warn("verify2fa: invalid code", "user_id", userID)
		http.Error(w, "invalid 2fa code", http.StatusUnauthorized)
		return
	}

	// 2FA успешна - выдаём полный access token
	var res *dto.AuthResult

	res, err = h.service.IssueTokensAfter2FA(ctx, userID)
	if err != nil {
		log.Error("verify2fa: issue tokens failed", "user_id", userID, "error", err)
		http.Error(w, "failed to issue tokens", http.StatusInternalServerError)
		return
	}
	log.Info("verify2fa: success", "user_id", userID)

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.AuthResponse{
		AccessToken: res.AccessToken,
		ExpiresIn:   res.ExpiresIn,
		Message:     "2FA verified",
	})
}

// POST /api/password/reset/request
func (h *UserHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("password-reset/request: invalid json", "error", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		log.Warn("password-reset/request: empty email")
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	err := h.service.RequestPasswordReset(r.Context(), req.Email)
	// Важно: не выдаём факт существования email.
	if err != nil && !errors.Is(err, my_errors.ErrUserNotFound) {
		log.Error("password-reset/request: failed", "email", req.Email, "error", err)
		http.Error(w, "failed to send reset email", http.StatusInternalServerError)
		return
	}
	log.Info("password-reset/request: accepted", "email", req.Email)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "if email exists, reset link sent"})
}

// POST /api/password/reset/confirm
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("password-reset/confirm: invalid json", "error", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		log.Warn("password-reset/confirm: missing fields")
		http.Error(w, "token and new_password required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 8 {
		log.Warn("password-reset/confirm: weak password")
		http.Error(w, "new_password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	userID, err := h.service.ValidateResetToken(r.Context(), req.Token)
	if err != nil {
		log.Warn("password-reset/confirm: invalid token", "error", err)
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}

	err = h.service.PerformPasswordReset(r.Context(), userID, req.NewPassword, req.Token)
	if err != nil {
		log.Error("password-reset/confirm: failed", "user_id", userID, "error", err)
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}
	log.Info("password-reset/confirm: success", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "password reset successful"})
}
