package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/response"
	"encoding/json"
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
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.CreateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("metrics/create: invalid request body", "error", err)
		response.SendBadRequest(w, "invalid request body")
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
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusCreated, toMetricsResponse(createdMetrics))
}

func (h *MetricHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/get: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var from, to *time.Time
	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			log.Warn("metrics/get: invalid from", "value", fromStr, "error", err)
			response.SendBadRequest(w, "invalid 'from' date format")
			return
		}
		from = &parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			log.Warn("metrics/get: invalid to", "value", toStr, "error", err)
			response.SendBadRequest(w, "invalid 'to' date format")
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
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, toMetricsResponse(metric))
}

func (h *MetricHandler) GetStoredMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/get-stored: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	filter := dto.MetricsFilter{UserID: userID}
	metric, err := h.service.GetAllMetrics(ctx, filter)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, toMetricsResponse(metric))
}

func (h *MetricHandler) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/update: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	var req dto.UpdateMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("metrics/update: invalid request body", "error", err)
		response.SendBadRequest(w, "invalid request body")
		return
	}

	if req.From != "" {
		if _, err := time.Parse("2006-01-02", req.From); err != nil {
			log.Warn("metrics/update: invalid from", "value", req.From, "error", err)
			response.SendBadRequest(w, "invalid 'from' date format")
			return
		}
	}
	if req.To != "" {
		if _, err := time.Parse("2006-01-02", req.To); err != nil {
			log.Warn("metrics/update: invalid to", "value", req.To, "error", err)
			response.SendBadRequest(w, "invalid 'to' date format")
			return
		}
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("metrics/update: invalid id", "value", idStr, "error", err)
		response.SendBadRequest(w, "invalid id")
		return
	}

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
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, toMetricsResponse(updatedMetric))
}

func (h *MetricHandler) DeleteMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	userID, ok := auth.GetUserID(ctx)
	if !ok {
		log.Warn("metrics/delete: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("metrics/delete: invalid id", "value", idStr, "error", err)
		response.SendBadRequest(w, "invalid id")
		return
	}

	if err := h.service.DeleteMetrics(ctx, id, userID); err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
