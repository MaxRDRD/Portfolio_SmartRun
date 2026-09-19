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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// DailyMetricHandler обрабатывает HTTP-запросы для ежедневных метрик
type DailyMetricHandler struct {
	service service.DailyMetricService
}

// NewDailyMetricHandler создает новый обработчик ежедневных метрик
func NewDailyMetricHandler(service service.DailyMetricService) *DailyMetricHandler {
	return &DailyMetricHandler{service: service}
}

// toDailyMetricResponse преобразует модель DailyMetric в DTO ответ
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
		SleepHours:     metric.SleepHours,
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

// parseDateYMD парсит дату из строки в формате ГГГГ-ММ-ДД
func parseDateYMD(raw string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parseDateYMD: %w", err)
	}
	return parsed.UTC(), nil
}

// CreateDailyMetric создает новую ежедневную метрику
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
		SleepHours:     req.SleepHours,
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

// GetDailyMetrics возвращает все ежедневные метрики пользователя
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

// GetDailyMetricByID возвращает ежедневную метрику по ее ID
func (h *DailyMetricHandler) GetDailyMetricByID(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		// Обратная совместимость для старых клиентов, использующих параметры запроса
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

// UpdateDailyMetric обновляет существующую ежедневную метрику
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

// DeleteDailyMetric удаляет ежедневную метрику по ее ID
func (h *DailyMetricHandler) DeleteDailyMetric(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		// Обратная совместимость для старых клиентов, использующих параметры запроса
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

// GenerateAdvice запускает процесс генерации советов на основе метрик
func (h *DailyMetricHandler) GenerateAdvice(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("daily-metrics/generate-advice: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Date string `json:"date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("daily-metrics/generate-advice: invalid body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	date, err := parseDateYMD(req.Date)
	if err != nil {
		log.Warn("daily-metrics/generate-advice: invalid date", "error", err)
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	if err := h.service.TriggerAdviceGeneration(r.Context(), userID, date); err != nil {
		log.Error("daily-metrics/generate-advice: failed to trigger", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
