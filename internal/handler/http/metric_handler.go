package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type MetricHandler struct {
	service service.MetricService
}

func NewMetricHandler(service service.MetricService) *MetricHandler {
	return &MetricHandler{service: service}
}

func (h *MetricHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var from, to *time.Time
	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			http.Error(w, "invalid 'from' date format", http.StatusBadRequest)
			return
		}
		from = &parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			http.Error(w, "invalid 'to' date format", http.StatusBadRequest)
			return
		}
		to = &parsed
	}

	filter := dto.MetricsFilter{
		UserID: userID,
		From:   from,
		To:     to,
	}

	metric, err := h.service.GetMetrics(ctx, filter)
	if err != nil {
		if errors.Is(err, my_errors.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(metric)

}
