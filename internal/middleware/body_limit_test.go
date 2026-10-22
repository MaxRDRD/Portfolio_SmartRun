package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appMiddleware "SmartRun/internal/middleware"
	"SmartRun/pkg/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyLimit_ContentLengthHeaderTooLarge(t *testing.T) {
	const maxBytes int64 = 100
	mw := appMiddleware.BodyLimit(maxBytes)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.Repeat("A", 200)
	req := httptest.NewRequest(http.MethodPost, "/info", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)

	var errResp response.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Request entity too large", errResp.Error)
	assert.Contains(t, errResp.Message, "exceeds maximum allowed limit")
}

func TestBodyLimit_StreamingBodyExceedsLimit(t *testing.T) {
	const maxBytes int64 = 50
	mw := appMiddleware.BodyLimit(maxBytes)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if assert.ErrorAs(t, err, &maxBytesErr) {
				response.SendRequestEntityTooLarge(w, "Request entity too large", "Exceeded limit")
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Chunked/streaming body with ContentLength = -1
	largePayload := bytes.Repeat([]byte("X"), 100)
	req := httptest.NewRequest(http.MethodPost, "/info", bytes.NewReader(largePayload))
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
