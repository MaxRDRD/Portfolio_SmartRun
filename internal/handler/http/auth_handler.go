package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"SmartRun/pkg/response"
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
		response.SendBadRequest(w, "invalid request body")
		return
	}

	res, err := h.service.Register(r.Context(), req)
	if err != nil {
		response.SendError(w, r, err)
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

	type registerResponse struct {
		User dto.UserResponse `json:"user"`
		Auth dto.AuthResponse `json:"auth"`
	}

	response.SendJSON(w, http.StatusCreated, registerResponse{
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
		response.SendBadRequest(w, "invalid request body")
		return
	}

	res, err := h.service.Login(r.Context(), req)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), res.User.ID)
	if err != nil {
		response.SendError(w, r, err)
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
		response.SendJSON(w, http.StatusAccepted, dto.AuthResponse{
			AccessToken: "", // пусто!
			ExpiresIn:   0,  // пусто!
			Require2FA:  true,
			Message:     "2FA required - call /verify-2fa with TOTP code",
		})
		return
	}

	// TOTP отключена - выдаём полный access token
	log.Info("login: success", "user_id", res.User.ID)
	response.SendJSON(w, http.StatusOK, dto.AuthResponse{
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
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	userResp, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	// проверка 2FA статуса
	totpEnabled, err := h.service.IsTOTPEnabled(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, dto.UserResponse{
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
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("update-me: invalid body", "error", err, "user_id", userID)
		response.SendBadRequest(w, "invalid request body")
		return
	}

	updated, err := h.service.UpdateUser(ctx, userID, req)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	totpEnabled, err := h.service.IsTOTPEnabled(ctx, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, dto.UserResponse{
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
		response.SendUnauthorized(w, "refresh token not found")
		return
	}

	var res *dto.AuthResult

	res, err = h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		// Важно: при любой ошибке (протух, отозван, подделан) → удаляем cookie
		log.Warn("refresh: invalid refresh token", "error", err)
		h.deleteRefreshCookie(ctx, w)
		response.SendUnauthorized(w, "invalid refresh token")
		return
	}

	// Успешно → ставим новый refresh
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    res.RefreshToken,
		Path:     "/api",
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production",
		SameSite: http.SameSiteStrictMode,
	})

	log.Info("refresh: success")
	response.SendJSON(w, http.StatusOK, dto.AuthResponse{
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

	response.SendJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

func (h *UserHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("enable2fa: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	email, err := h.service.GetEmailByID(ctx, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	secret, qrBytes, err := h.service.EnableTOTP(ctx, userID, email)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	log.Info("enable2fa: success", "user_id", userID)

	resp := dto.Enable2FAResponse{
		QRBase64: base64.StdEncoding.EncodeToString(qrBytes),
		Secret:   secret, // ← только здесь отдаём, потом уже никогда
	}

	response.SendJSON(w, http.StatusOK, resp)
}

func (h *UserHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Требуем refresh token из cookie для идентификации (БЕЗ JWT requirement)
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		log.Warn("verify2fa: refresh cookie required", "error", err)
		response.SendUnauthorized(w, "refresh_token cookie required")
		return
	}

	// Валидируем refresh token и получаем userID из сессии
	refreshHash := auth.HashToken(cookie.Value)
	session, err := h.service.GetSessionByHash(ctx, refreshHash)
	if err != nil {
		log.Info("invalid or expired session")
		response.SendUnauthorized(w, "invalid or expired session")
		return
	}
	userID := session.UserId

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("verify2fa: invalid json", "error", err)
		response.SendBadRequest(w, "invalid json")
		return
	}

	if req.Code == "" {
		log.Warn("verify2fa: empty code", "user_id", userID)
		response.SendBadRequest(w, "code required")
		return
	}

	// Проверяем TOTP code
	valid, err := h.service.VerifyTOTP(ctx, userID, req.Code)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	if !valid {
		log.Warn("verify2fa: invalid code", "user_id", userID)
		response.SendUnauthorized(w, "invalid 2fa code")
		return
	}

	// 2FA успешна - выдаём полный access token
	var res *dto.AuthResult

	res, err = h.service.IssueTokensAfter2FA(ctx, userID)
	if err != nil {
		response.SendError(w, r, err)
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

	response.SendJSON(w, http.StatusOK, dto.AuthResponse{
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
		response.SendBadRequest(w, "invalid json")
		return
	}
	if req.Email == "" {
		log.Warn("password-reset/request: empty email")
		response.SendBadRequest(w, "email required")
		return
	}

	err := h.service.RequestPasswordReset(r.Context(), req.Email)
	// Важно: не выдаём факт существования email.
	if err != nil && !errors.Is(err, my_errors.ErrUserNotFound) {
		response.SendError(w, r, err)
		return
	}
	log.Info("password-reset/request: accepted", "email", req.Email)

	response.SendJSON(w, http.StatusOK, map[string]string{"message": "if email exists, reset link sent"})
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
		response.SendBadRequest(w, "invalid json")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		log.Warn("password-reset/confirm: missing fields")
		response.SendBadRequest(w, "token and new_password required")
		return
	}
	if len(req.NewPassword) < 8 {
		log.Warn("password-reset/confirm: weak password")
		response.SendBadRequest(w, "new_password must be at least 8 characters")
		return
	}

	userID, err := h.service.ValidateResetToken(r.Context(), req.Token)
	if err != nil {
		log.Warn("password-reset/confirm: invalid token", "error", err)
		response.SendBadRequest(w, "invalid or expired token")
		return
	}

	err = h.service.PerformPasswordReset(r.Context(), userID, req.NewPassword, req.Token)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	log.Info("password-reset/confirm: success", "user_id", userID)

	response.SendJSON(w, http.StatusOK, map[string]string{"message": "password reset successful"})
}
