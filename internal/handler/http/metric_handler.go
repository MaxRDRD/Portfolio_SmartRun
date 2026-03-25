package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type MetricHandler struct {
	service service.MetricService
}

func NewMetricHandler(service service.MetricService) *MetricHandler {
	return &MetricHandler{service: service}
}

func toMetricsResponse(metric *model.Metrics) dto.MetricsResponse {
	if metric == nil {
		return dto.MetricsResponse{}
	}

	return dto.MetricsResponse{
		ID:            metric.ID,
		UserID:        metric.UserID,
		TotalWorkouts: metric.TotalWorkouts,
		TotalDistance: metric.TotalDistance,
		TotalDuration: metric.TotalDuration,
		AvgPace:       metric.AvgPace,
		TotalCalories: metric.TotalCalories,
		From:          metric.From,
		To:            metric.To,
	}
}

func (h *MetricHandler) CreateMetrics(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	var req dto.CreateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	metrics := model.Metrics{
		UserID:        userID,
		TotalWorkouts: req.TotalWorkouts,
		TotalDistance: req.TotalDistance,
		TotalDuration: req.TotalDuration,
		AvgPace:       req.AvgPace,
		From:          req.From,
		To:            req.To,
		TotalCalories: req.TotalCalories,
	}

	createdMetrics, err := h.service.CreateMetrics(r.Context(), metrics)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toMetricsResponse(createdMetrics)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toMetricsResponse(metric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

}

func (h *MetricHandler) GetStoredMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	filter := dto.MetricsFilter{UserID: userID}
	metric, err := h.service.GetAllMetrics(ctx, filter)
	if err != nil {
		if errors.Is(err, my_errors.ErrMetricNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toMetricsResponse(metric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

}

func (h *MetricHandler) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var req dto.UpdateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.From != "" {
		if _, err := time.Parse("2006-01-02", req.From); err != nil {
			http.Error(w, "invalid 'from' date format", http.StatusBadRequest)
			return
		}
	}
	if req.To != "" {
		if _, err := time.Parse("2006-01-02", req.To); err != nil {
			http.Error(w, "invalid 'to' date format", http.StatusBadRequest)
			return
		}
	}

	metrics := model.Metrics{
		UserID:        userID,
		TotalWorkouts: req.TotalWorkouts,
		TotalDistance: req.TotalDistance,
		TotalDuration: req.TotalDuration,
		AvgPace:       req.AvgPace,
		From:          req.From,
		To:            req.To,
		TotalCalories: req.TotalCalories,
	}

	updatedMetric, err := h.service.UpdateMetrics(ctx, metrics)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toMetricsResponse(updatedMetric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

}

func (h *MetricHandler) DeleteMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	if idStr := r.URL.Query().Get("id"); idStr != "" {
		if _, err := strconv.Atoi(idStr); err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
	}

	if err := h.service.DeleteMetrics(ctx, int(userID)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)

}
