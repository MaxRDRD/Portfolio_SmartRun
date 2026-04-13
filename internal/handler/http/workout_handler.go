package http

import (
	"bytes"
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/mapper"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type WorkoutHandler struct {
	service service.WorkoutService
}

func NewWorkoutHandler(service service.WorkoutService) *WorkoutHandler {
	return &WorkoutHandler{service: service}
}

func (h *WorkoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/create: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	const maxUploadSize = 5 * 1024 * 1024 // 5 МБ — больше FIT-файлов почти не бывает

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")

	var workout *model.Workouts
	var err error

	if strings.HasPrefix(contentType, "application/json") {
		var req dto.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Warn("workouts/create: invalid json", "error", err)
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		workout, err = h.service.Create(r.Context(), userID, req)

	} else if contentType == "" ||
		strings.HasPrefix(contentType, "application/octet-stream") ||
		strings.HasPrefix(contentType, "application/vnd.garmin.fit") {
		// Загрузка FIT файла как бинарного потока
		data, err := io.ReadAll(r.Body)
		if len(data) == 0 {
			log.Warn("workouts/create: empty fit file")
			http.Error(w, "empty file", http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Warn("workouts/create: failed to read fit file", "error", err)
			http.Error(w, "failed to read file", http.StatusBadRequest)
			return
		}
		workout, err = h.service.UploadFit(r.Context(), userID, data)

	} else {
		log.Warn("workouts/create: unsupported content type", "content_type", contentType)
		http.Error(w, "unsupported Content-Type: "+contentType, http.StatusUnsupportedMediaType)
		return
	}

	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutAlreadyExists) {
			log.Warn("workouts/create: already exists", "user_id", userID)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		log.Error("workouts/create: service failed", "error", err, "user_id", userID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workout == nil || workout.ID <= 0 {
		log.Error("workouts/create: success path without persisted workout id", "user_id", userID)
		http.Error(w, "workout was not persisted", http.StatusInternalServerError)
		return
	}

	response := mapper.ToWorkoutsResponse(workout)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		log.Error("workouts/create: failed to encode response", "error", err, "user_id", userID, "workout_id", workout.ID)
		http.Error(w, "failed to encode workout response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Error("workouts/create: failed to write response", "error", err, "user_id", userID, "workout_id", workout.ID)
	}
}

/*
	ID              int64    `json:"id"`
    Date            string   `json:"date"`
    TypeActivity    string   `json:"type_activity"`
    Distance        float64  `json:"distance"`
    Duration        int      `json:"duration"`
    Pace            float64  `json:"pace"` // или string "4:19"
    AvgCadence      *int     `json:"avg_cadence"`
    ElevationGain   *float64 `json:"elevation_gain"`
    AvgHR           *int     `json:"avg_hr"`
    MaxHR           *int     `json:"max_hr"`
    Calories        *int     `json:"calories"`
    VO2MaxEstimate  *float64 `json:"vo2max_estimate,omitempty"`
    RecoveryTime    *int     `json:"recovery_time,omitempty"`
    TrainingLoad    *float64 `json:"training_load,omitempty"`
    PerceivedEffort int      `json:"perceived_effort,omitempty"`
    Notes           string   `json:"notes,omitempty"`
    Shoes           string   `json:"shoes,omitempty"`
*/

func (h *WorkoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	var req dto.UpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("workouts/update: invalid body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/update: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/update: invalid id", "value", idStr, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	isFullReplace := r.Method == http.MethodPut
	workout, err := h.service.Update(r.Context(), userID, id, req, isFullReplace)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutNotFound) {
			log.Warn("workouts/update: not found", "id", id, "user_id", userID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("workouts/update: service failed", "id", id, "user_id", userID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(mapper.ToWorkoutsResponse(workout))
}

func (h *WorkoutHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	filter, err := buildWorkoutFilterFromQuery(r, userID)
	if err != nil {
		var badReqErr *queryParamError
		if errors.As(err, &badReqErr) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "invalid query params", http.StatusBadRequest)
		return
	}

	workouts, err := h.service.GetAllByID(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responses := make([]dto.WorkoutsResponse, len(workouts))
	for i, w := range workouts {
		responses[i] = mapper.ToWorkoutsResponse(&w)
	}
	json.NewEncoder(w).Encode(responses)
}

func (h *WorkoutHandler) GetHistoryByMonth(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	monthsLimit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	monthsOffset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if monthsLimit <= 0 {
		monthsLimit = 3
	}
	if monthsLimit > 12 {
		monthsLimit = 12
	}
	if monthsOffset < 0 {
		monthsOffset = 0
	}

	history, err := h.service.GetMonthlyHistory(r.Context(), userID, monthsLimit, monthsOffset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(mapper.ToWorkoutHistoryResponse(history))
}

func (h *WorkoutHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/get-by-id: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/get-by-id: invalid id", "value", idStr, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	workout, err := h.service.GetByID(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutNotFound) {
			log.Warn("workouts/get-by-id: not found", "id", id, "user_id", userID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("workouts/get-by-id: service failed", "id", id, "user_id", userID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(mapper.ToWorkoutsResponse(workout))
}

func (h *WorkoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/delete: unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/delete: invalid id", "value", idStr, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = h.service.Delete(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutNotFound) {
			log.Warn("workouts/delete: not found", "id", id, "user_id", userID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Error("workouts/delete: service failed", "id", id, "user_id", userID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
