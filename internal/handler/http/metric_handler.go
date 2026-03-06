package http

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"context"
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
	ctx := context.Background()

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	userID := r.Context().Value("user_id").(int)

	var from, to *time.Time
	if fromStr != "" {
		parsed, _ := time.Parse("2006-01-02", fromStr)
		from = &parsed
	}
	if toStr != "" {
		parsed, _ := time.Parse("2006-01-02", toStr)
		to = &parsed
	}

	filter := dto.MetricsFilter{
		UserID: userID,
		From:   from,
		To:     to,
	}

	metrics, err := h.service.GetMetrics(ctx, filter)
	if errors.Is(err, my_errors.ErrMetricNotFound) {
		http.Error(w, err.Error(), http.StatusConflict)
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)

	json.NewEncoder(w).Encode(metrics)

}
