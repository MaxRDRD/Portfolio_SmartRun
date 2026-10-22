package response

import (
	"SmartRun/internal/logger"
	"SmartRun/pkg/apperrors"

	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`   // машиночитаемый код
	Message string `json:"message"` // сообщение
}

// helper func для отправки ошибок в JSON
func SendJSON(w http.ResponseWriter, status int, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// helper func для отправки ошибок в JSON
func SendError(w http.ResponseWriter, r *http.Request, err error) {
	log := logger.FromContext(r.Context())
	status := apperrors.HTTPStatus(err)
	var resp ErrorResponse
	switch status {
	case http.StatusInternalServerError: // 500
		// Логируем детальную внутреннюю ошибку со всем контекстом
		log.Error("internal server error", "error", err, "path", r.URL.Path, "method", r.Method)
		// Клиенту отдаём безопасный ответ без утечки деталей базы данных
		resp = ErrorResponse{
			Error:   "internal_error",
			Message: "internal server error",
		}
	case http.StatusNotFound: // 404
		log.Warn("resource not found", "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		}
	case http.StatusConflict: // 409
		log.Warn("resource conflict", "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "conflict",
			Message: err.Error(),
		}
	case http.StatusUnauthorized: // 401
		log.Warn("unauthorized request", "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "unauthorized",
			Message: err.Error(),
		}
	case http.StatusBadRequest: // 400
		log.Warn("bad request", "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "bad_request",
			Message: err.Error(),
		}
	case http.StatusRequestEntityTooLarge:
		log.Warn("request entity too large", "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "Request entity too large",
			Message: err.Error(),
		}
	default:
		log.Warn("client error", "status", status, "error", err, "path", r.URL.Path)
		resp = ErrorResponse{
			Error:   "error",
			Message: err.Error(),
		}
	}
	// Отправляем сформированный ответ
	SendJSON(w, status, resp)
}

// SendBadRequest отправляет 400 Bad Request в формате JSON
func SendBadRequest(w http.ResponseWriter, message string) {
	SendJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "bad_request",
		Message: message,
	})
}

// SendUnauthorized отправляет 401 Unauthorized в формате JSON
func SendUnauthorized(w http.ResponseWriter, message string) {
	SendJSON(w, http.StatusUnauthorized, ErrorResponse{
		Error:   "unauthorized",
		Message: message,
	})
}

// SendRequestEntityTooLarge отправляет 413 Request Entity Too Large в формате JSON
func SendRequestEntityTooLarge(w http.ResponseWriter, message, details string) {
	SendJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{
		Error:   "request_entity_too_large",
		Message: message,
	})
}
