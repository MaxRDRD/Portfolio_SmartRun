package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
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
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("metrics/create: unauthorized")
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	log = log.With("user_id", userID)
	var req dto.CreateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("metrics/create: invalid request body", "error", err)
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
		log.Error("metrics/create: service failed", "error", err)
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
	log := logger.FromContext(ctx)

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/get: unauthorized")
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	log = log.With("user_id", userID)

	var from, to *time.Time
	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			log.Warn("metrics/get: invalid from", "value", fromStr, "error", err)
			http.Error(w, "invalid 'from' date format", http.StatusBadRequest)
			return
		}
		from = &parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			log.Warn("metrics/get: invalid to", "value", toStr, "error", err)
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
			log.Warn("metrics/get: not found")
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("metrics/get: service failed", "error", err)
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
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/get-stored: unauthorized")
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	log = log.With("user_id", userID)

	filter := dto.MetricsFilter{UserID: userID}
	metric, err := h.service.GetAllMetrics(ctx, filter)
	if err != nil {
		if errors.Is(err, my_errors.ErrMetricNotFound) {
			log.Warn("metrics/get-stored: not found")
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("metrics/get-stored: service failed", "error", err)
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
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/update: unauthorized")
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	var req dto.UpdateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("metrics/update: invalid request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.From != "" {
		if _, err := time.Parse("2006-01-02", req.From); err != nil {
			log.Warn("metrics/update: invalid from", "value", req.From, "error", err)
			http.Error(w, "invalid 'from' date format", http.StatusBadRequest)
			return
		}
	}
	if req.To != "" {
		if _, err := time.Parse("2006-01-02", req.To); err != nil {
			log.Warn("metrics/update: invalid to", "value", req.To, "error", err)
			http.Error(w, "invalid 'to' date format", http.StatusBadRequest)
			return
		}
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("metrics/update: invalid id", "value", idStr, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	log = log.With("user_id", userID, "metrics_id", id)

	metrics := model.Metrics{
		ID:            id,
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
		log.Error("metrics/update: service failed", "error", err)
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
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/delete: unauthorized")
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("metrics/delete: invalid id", "value", idStr, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	log = log.With("user_id", userID, "metrics_id", id)

	if err := h.service.DeleteMetrics(ctx, id, userID); err != nil {
		log.Error("metrics/delete: service failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)

}
