package http

import (
	"SmartRun/internal/auth"
	"SmartRun/internal/dto"
	"SmartRun/internal/logger"
	"SmartRun/internal/mapper"
	"SmartRun/internal/model"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"SmartRun/pkg/response"
	"encoding/json"
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
		response.SendUnauthorized(w, "unauthorized")
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
			response.SendBadRequest(w, "invalid JSON body")
			return
		}
		workout, err = h.service.Create(r.Context(), userID, req)
	} else if contentType == "" ||
		strings.HasPrefix(contentType, "application/octet-stream") ||
		strings.HasPrefix(contentType, "application/vnd.garmin.fit") {
		// Загрузка FIT файла как бинарного потока
		var data []byte
		data, err = io.ReadAll(r.Body)
		if err != nil {
			log.Warn("workouts/create: failed to read fit file", "error", err)
			response.SendBadRequest(w, "failed to read file")
			return
		}
		if len(data) == 0 {
			log.Warn("workouts/create: empty fit file")
			response.SendBadRequest(w, "empty file")
			return
		}
		workout, err = h.service.UploadFit(r.Context(), userID, data)

	} else {
		log.Warn("workouts/create: unsupported content type", "content_type", contentType)
		response.SendJSON(w, http.StatusUnsupportedMediaType, response.ErrorResponse{
			Error:   "unsupported_media_type",
			Message: "unsupported Content-Type: " + contentType,
		})
		return
	}

	if err != nil {
		response.SendError(w, r, err)
		return
	}
	if workout == nil || workout.ID <= 0 {
		log.Error("workouts/create: success path without persisted workout id", "user_id", userID)
		response.SendError(w, r, my_errors.ErrInternal)
		return
	}

	response.SendJSON(w, http.StatusCreated, mapper.ToWorkoutsResponse(workout))
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
		response.SendBadRequest(w, "invalid request body")
		return
	}

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/update: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/update: invalid id", "value", idStr, "error", err)
		response.SendBadRequest(w, "invalid id")
		return
	}
	isFullReplace := r.Method == http.MethodPut
	workout, err := h.service.Update(r.Context(), userID, id, req, isFullReplace)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, mapper.ToWorkoutsResponse(workout))
}

func (h *WorkoutHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	filter, err := buildWorkoutFilterFromQuery(r, userID)
	if err != nil {
		response.SendBadRequest(w, err.Error())
		return
	}

	workouts, err := h.service.GetAllByID(r.Context(), filter)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	responses := make([]dto.WorkoutsResponse, len(workouts))
	for i, w := range workouts {
		responses[i] = mapper.ToWorkoutsResponse(&w)
	}
	response.SendJSON(w, http.StatusOK, responses)
}

func (h *WorkoutHandler) GetHistoryByMonth(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		response.SendUnauthorized(w, "unauthorized")
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
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, mapper.ToWorkoutHistoryResponse(history))
}

func (h *WorkoutHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/get-by-id: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/get-by-id: invalid id", "value", idStr, "error", err)
		response.SendBadRequest(w, "invalid id")
		return
	}

	workout, err := h.service.GetByID(r.Context(), id, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	response.SendJSON(w, http.StatusOK, mapper.ToWorkoutsResponse(workout))
}

func (h *WorkoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		log.Warn("workouts/delete: unauthorized")
		response.SendUnauthorized(w, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Warn("workouts/delete: invalid id", "value", idStr, "error", err)
		response.SendBadRequest(w, "invalid id")
		return
	}

	err = h.service.Delete(r.Context(), id, userID)
	if err != nil {
		response.SendError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
