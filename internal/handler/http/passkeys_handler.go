package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/response"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type PasskeyHandler struct {
	service service.PasskeyService
}

func NewPasskeyHandler(service service.PasskeyService) *PasskeyHandler {
	return &PasskeyHandler{service: service}
}

func (h *PasskeyHandler) BeginRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Проверяем существование пользователя
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("passkey/BeginRegistration: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	// Генерируем вызов (Challenge)
	createdPasskey, err := h.service.BeginRegistration(r.Context(), userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, createdPasskey)

}

func (h *PasskeyHandler) FinishRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Проверяем существование пользователя
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("passkey/FinishRegistration: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	// Пытаемся достать имя устройства пользователя
	deviceName := r.URL.Query().Get("device_name")
	if deviceName == "" {
		// Если имя пустое - присваиваем свое значение для идентификации
		deviceName = "PasskeyDevice"
	}
	// Завершаем регистрацию ключа
	pk, err := h.service.FinishRegistration(ctx, userID, r.Body, deviceName)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// Маппим ответ в dto struct и отдаем пользователю
	dtoPK := dto.PasskeyResponse{
		ID:         pk.ID,
		DeviceName: pk.DeviceName,
		CreatedAt:  pk.CreatedAt,
		LastUsedAt: pk.LastUsedAt,
	}

	response.SendJSON(w, http.StatusCreated, dtoPK)
}

func (h *PasskeyHandler) BeginLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Пытаемся достать ключ из тела запроса
	var req dto.PasskeyLoginBeginRequest
	if r.Body != nil && r.Body != http.NoBody {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			log.Warn("passkey/login/begin: invalid body", "error", err)
			response.SendBadRequest(w, "invalid request body")
			return
		}
	}
	// Проводим авторизацию
	assertion, sessionID, err := h.service.BeginLogin(ctx, req.Email)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// Маппим ответ в dto struct и отдаем пользователю
	resp := dto.PasskeyLoginBeginResponse{
		SessionID: sessionID,
		Options:   assertion,
	}
	response.SendJSON(w, http.StatusOK, resp)
}

func (h *PasskeyHandler) FinishLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Извлекаем сессию пользователя
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
		if sessionID == "" {
			response.SendBadRequest(w, "missing session_id")
			return
		}
	}
	// Завершаем авторизацию пользователя
	authResult, err := h.service.FinishLogin(ctx, sessionID, r.Body)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// Устанавливаем HttpOnly cookie с refresh-токеном
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    authResult.RefreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30, // 30 дней
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") == "production", // только HTTPS если в prod - в dev false
		SameSite: http.SameSiteStrictMode,
	})

	log.Info("login: success", "user_id", authResult.User.ID)
	response.SendJSON(w, http.StatusOK, dto.AuthResponse{
		AccessToken: authResult.AccessToken,
		ExpiresIn:   authResult.ExpiresIn,
		Message:     "ok",
	})
}

func (h *PasskeyHandler) GetUserPasskeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Проверяем существование пользователя
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("passkey/GetUserPasskeys: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	// Находим все ключи, отведенные под данного пользователя
	list, err := h.service.GetUserPasskeys(ctx, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}
	// Маппим найденные ключи в dto struct и отдаем безопасные данные
	dtoList := make([]dto.PasskeyResponse, len(list))
	for i, l := range list {
		dtoList[i].ID = l.ID
		dtoList[i].DeviceName = l.DeviceName
		dtoList[i].CreatedAt = l.CreatedAt
		dtoList[i].LastUsedAt = l.LastUsedAt
	}

	response.SendJSON(w, http.StatusOK, dtoList)
}

func (h *PasskeyHandler) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	// Проверяем существование пользователя
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("passkey/DeletePasskey: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}
	// Ищем id ключа
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("passkey/DeletePasskey: invalid id", "id", idStr, "error", err)
		response.SendBadRequest(w, "invalid passkey id")
		return
	}
	// Удаляем ключ
	err = h.service.DeletePasskey(ctx, id, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
