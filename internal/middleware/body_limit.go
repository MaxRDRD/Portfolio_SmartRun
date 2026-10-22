package middleware

import (
	"fmt"
	"net/http"

	"SmartRun/pkg/response"
)

// BodyLimit ограничивает максимальный размер тела входящего запроса в байтах
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Быстрая проверка заголовка Content-Length перед чтением тела
			if r.ContentLength > maxBytes {
				response.SendJSON(w, http.StatusRequestEntityTooLarge, response.ErrorResponse{
					Error:   "Request entity too large",
					Message: fmt.Sprintf("Request body size (%d bytes) exceeds maximum allowed limit of %d bytes", r.ContentLength, maxBytes),
				})
				return
			}

			// Ограничиваем поток чтения тела на случай отсутствия заголовка Content-Length
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}
