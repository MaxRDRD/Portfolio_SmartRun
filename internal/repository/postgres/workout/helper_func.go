package postgres

import (
	"strings"
)

func toInt32Slice(values []int) []int32 {
	if len(values) == 0 {
		return nil
	}

	out := make([]int32, len(values))
	for i, v := range values {
		out[i] = int32(v)
	}
	return out
}

func toIntSlice(values []int32) []int {
	if len(values) == 0 {
		return nil
	}

	out := make([]int, len(values))
	for i, v := range values {
		out[i] = int(v)
	}
	return out
}

func defaultWorkoutPreviewImage(typeActivity string) string {
	activity := strings.ToLower(strings.TrimSpace(typeActivity))

	switch activity {
	case "run", "running":
		return "/assets/workouts/running.jpg"
	case "trail_run", "trail":
		return "/assets/workouts/trail.jpg"
	case "bike", "cycling":
		return "/assets/workouts/cycling.jpg"
	case "swim", "swimming":
		return "/assets/workouts/swimming.jpg"
	default:
		return "/assets/workouts/default.jpg"
	}
}
