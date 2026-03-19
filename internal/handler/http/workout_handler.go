package http

import (
	"SmartRun/internal/dto"
	"SmartRun/internal/usecase/service"
	"SmartRun/pkg/my_errors"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
)

type WorkoutHandler struct {
	service service.WorkoutService
}

func NewWorkoutHandler(service service.WorkoutService) *WorkoutHandler {
	return &WorkoutHandler{service: service}
}

func (h *WorkoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	userId := r.Context().Value("user_id").(int)

	workout, err := h.service.Create(r.Context(), userId, req)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(dto.WorkoutsResponse{
		ID:           workout.ID,
		Date:         workout.Date.String(),
		TypeActivity: workout.TypeActivity,
		Distance:     workout.Distance,
		Duration:     workout.Duration,
	})
}

func (h *WorkoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("user_id").(int)

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	workout, err := h.service.Update(r.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(workout)
}

func (h *WorkoutHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	typeActive := r.URL.Query().Get("type")

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to *time.Time

	if fromStr != "" {
		parsed, _ := time.Parse("2006-01-02", fromStr)
		from = &parsed
	}
	if toStr != "" {
		parsed, _ := time.Parse("2006-01-02", toStr)
		to = &parsed
	}

	filter := dto.WorkoutFilter{
		UserID: userID,
		Type:   typeActive,
		From:   from,
		To:     to,
		Limit:  limit,
		Offset: offset,
	}

	workouts, err := h.service.GetAll(r.Context(), filter)
	if err != nil {
		if errors.Is(err, my_errors.ErrWorkoutNotFound) {
			http.Error(w, err.Error(), http.StatusConflict)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	json.NewEncoder(w).Encode(workouts)
}
