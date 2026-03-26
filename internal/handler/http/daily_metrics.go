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

type DailyMetricHandler struct {
	service service.DailyMetricService
}

func NewDailyMetricHandler(service service.DailyMetricService) *DailyMetricHandler {
	return &DailyMetricHandler{service: service}
}

func toDailyMetricResponse(metric *model.DailyMetric) dto.DailyMetricResponse {
	if metric == nil {
		return dto.DailyMetricResponse{}
	}

	res := dto.DailyMetricResponse{
		ID:             metric.ID,
		UserID:         metric.UserID,
		Date:           metric.Date.UTC().Format("2006-01-02"),
		CTL:            metric.CTL,
		ATL:            metric.ATL,
		TSB:            metric.TSB,
		FatigueScore:   metric.FatigueScore,
		ReadinessScore: metric.ReadinessScore,
		BodyBatteryAvg: metric.BodyBatteryAvg,
		Steps:          metric.Steps,
		TotalCalories:  metric.TotalCalories,
		SleepScore:     metric.SleepScore,
		StressAvg:      metric.StressAvg,
		Recommendation: metric.Recommendation,
		StreakDays:     metric.StreakDays,
		Monotony:       metric.Monotony,
		Strain:         metric.Strain,
	}

	if !metric.UpdatedAt.IsZero() {
		updatedAt := metric.UpdatedAt.UTC()
		res.UpdatedAt = &updatedAt
	}

	return res
}

func parseDateYMD(raw string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func (h *DailyMetricHandler) CreateDailyMetric(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("daily-metrics/create: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.CreateDailyMetricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("daily-metrics/create: invalid body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	metricDate := time.Now().UTC()
	if req.Date != "" {
		parsedDate, err := parseDateYMD(req.Date)
		if err != nil {
			log.Warn("daily-metrics/create: invalid date", "value", req.Date, "error", err)
			http.Error(w, "invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		metricDate = parsedDate
	}

	dailyMetric := model.DailyMetric{
		UserID:         userID,
		Date:           metricDate,
		SleepScore:     req.SleepScore,
		BodyBatteryAvg: req.BodyBatteryAvg,
		Steps:          req.Steps,
	}

	createdMetric, err := h.service.CreateDailyMetric(r.Context(), dailyMetric)
	if err != nil {
		if errors.Is(err, my_errors.ErrDailyMetricAlreadyExists) {
			log.Warn("daily-metrics/create: already exists", "user_id", userID, "date", req.Date)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		log.Error("daily-metrics/create: service failed", "error", err, "user_id", userID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toDailyMetricResponse(createdMetric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *DailyMetricHandler) GetDailyMetrics(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("daily-metrics/get-all: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	metrics, err := h.service.GetAllDailyMetrics(r.Context(), userID)
	if err != nil {
		if errors.Is(err, my_errors.ErrDailyMetricNotFound) {
			log.Warn("daily-metrics/get-all: not found", "user_id", userID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("daily-metrics/get-all: service failed", "error", err, "user_id", userID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := make([]dto.DailyMetricResponse, 0, len(metrics))
	for i := range metrics {
		m := metrics[i]
		resp = append(resp, toDailyMetricResponse(&m))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *DailyMetricHandler) GetDailyMetricByID(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		// Backward compatibility for old clients using query-param style.
		idStr = r.URL.Query().Get("id")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Warn("daily-metrics/get-by-id: invalid id", "value", idStr, "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	metric, err := h.service.GetDailyMetricByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, my_errors.ErrDailyMetricNotFound) {
			log.Warn("daily-metrics/get-by-id: not found", "id", id)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("daily-metrics/get-by-id: service failed", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toDailyMetricResponse(metric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *DailyMetricHandler) UpdateDailyMetric(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("daily-metrics/update: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.UpdateDailyMetricRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("daily-metrics/update: invalid body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	req.UserID = userID

	updatedMetric, err := h.service.UpdateDailyMetric(r.Context(), req)
	if err != nil {
		if errors.Is(err, my_errors.ErrDailyMetricNotFound) {
			log.Warn("daily-metrics/update: not found", "id", req.ID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("daily-metrics/update: service failed", "id", req.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toDailyMetricResponse(updatedMetric)); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *DailyMetricHandler) DeleteDailyMetric(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		// Backward compatibility for old clients using query-param style.
		idStr = r.URL.Query().Get("id")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Warn("daily-metrics/delete: invalid id", "value", idStr, "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	err = h.service.DeleteDailyMetric(r.Context(), id)
	if err != nil {
		if errors.Is(err, my_errors.ErrDailyMetricNotFound) {
			log.Warn("daily-metrics/delete: not found", "id", id)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("daily-metrics/delete: service failed", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
