package http

import (
	"SmartRun/internal/dto"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type queryParamError struct {
	Param string
	Value string
	Msg   string
}

func (e *queryParamError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("invalid query param '%s': %s", e.Param, e.Msg)
	}
	return fmt.Sprintf("invalid query param '%s'='%s': %s", e.Param, e.Value, e.Msg)
}

func buildWorkoutFilterFromQuery(r *http.Request, userID int64) (dto.WorkoutFilter, error) {
	q := r.URL.Query()

	from, err := parseDatePtr(q.Get("from"), "from")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	to, err := parseDatePtr(q.Get("to"), "to")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	minDistance, err := parseFloatPtr(q.Get("min_distance"), "min_distance")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	maxDistance, err := parseFloatPtr(q.Get("max_distance"), "max_distance")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	minDuration, err := parseIntPtr(q.Get("min_duration"), "min_duration")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	maxDuration, err := parseIntPtr(q.Get("max_duration"), "max_duration")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	minAvgHR, err := parseIntPtr(q.Get("min_avg_hr"), "min_avg_hr")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	maxAvgHR, err := parseIntPtr(q.Get("max_avg_hr"), "max_avg_hr")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	minPace, err := parseFloatPtr(q.Get("min_pace"), "min_pace")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	maxPace, err := parseFloatPtr(q.Get("max_pace"), "max_pace")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	minRPE, err := parseIntPtr(q.Get("min_rpe"), "min_rpe")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	maxRPE, err := parseIntPtr(q.Get("max_rpe"), "max_rpe")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	hasNotes, err := parseBoolPtr(q.Get("has_notes"), "has_notes")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	hasHRZones, err := parseBoolPtr(q.Get("has_hr_zones"), "has_hr_zones")
	if err != nil {
		return dto.WorkoutFilter{}, err
	}

	limit, err := parseIntWithDefault(q.Get("limit"), "limit", 20)
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	if limit <= 0 {
		return dto.WorkoutFilter{}, &queryParamError{Param: "limit", Value: q.Get("limit"), Msg: "must be > 0"}
	}

	offset, err := parseIntWithDefault(q.Get("offset"), "offset", 0)
	if err != nil {
		return dto.WorkoutFilter{}, err
	}
	if offset < 0 {
		return dto.WorkoutFilter{}, &queryParamError{Param: "offset", Value: q.Get("offset"), Msg: "must be >= 0"}
	}

	return dto.WorkoutFilter{
		UserID:      userID,
		Type:        q.Get("type"),
		From:        from,
		To:          to,
		MinDistance: minDistance,
		MaxDistance: maxDistance,
		MinDuration: minDuration,
		MaxDuration: maxDuration,
		MinAvgHR:    minAvgHR,
		MaxAvgHR:    maxAvgHR,
		MinPace:     minPace,
		MaxPace:     maxPace,
		MinRPE:      minRPE,
		MaxRPE:      maxRPE,
		HasNotes:    hasNotes,
		HasHRZones:  hasHRZones,
		Shoes:       q.Get("shoes"),
		Limit:       limit,
		Offset:      offset,
		SortBy:      q.Get("sort_by"),
		SortOrder:   q.Get("sort_order"),
	}, nil
}

func parseDatePtr(raw, param string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, &queryParamError{Param: param, Value: raw, Msg: "expected format YYYY-MM-DD"}
	}
	return &v, nil
}

func parseFloatPtr(raw, param string) (*float64, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, &queryParamError{Param: param, Value: raw, Msg: "must be a number"}
	}
	return &v, nil
}

func parseIntPtr(raw, param string) (*int, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, &queryParamError{Param: param, Value: raw, Msg: "must be an integer"}
	}
	return &v, nil
}

func parseBoolPtr(raw, param string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, &queryParamError{Param: param, Value: raw, Msg: "must be true/false"}
	}
	return &v, nil
}

func parseIntWithDefault(raw, param string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &queryParamError{Param: param, Value: raw, Msg: "must be an integer"}
	}
	return v, nil
}
