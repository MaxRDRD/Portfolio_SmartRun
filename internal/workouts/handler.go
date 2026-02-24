package workouts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	userId := r.Context().Value("user_id").(int)

	workout, err := h.service.Create(r.Context(), userId, req)
	if err != nil {
		if errors.Is(err, ErrWorkoutAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(WorkoutsResponse{
		ID:           workout.ID,
		Date:         workout.Date.String(),
		TypeActivity: workout.TypeActivity,
		Distance:     workout.Distance,
		Duration:     workout.Duration,
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("user_id").(int)

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	workout, err := h.service.Update(r.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, ErrWorkoutNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(workout)
}
