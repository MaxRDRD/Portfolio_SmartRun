package postgres

import (
	"SmartRun/internal/dto"
	"fmt"
	"strings"
)

// buildWorkoutFilters строит WHERE условия для фильтра.
func (r *workoutRepository) buildWorkoutFilters(filter dto.WorkoutFilter) (string, []interface{}, int) {
	var conditions []string
	args := []interface{}{filter.UserID}
	argPos := 2

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type_activity = $%d", argPos))
		args = append(args, filter.Type)
		argPos++
	}

	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("date >= $%d", argPos))
		args = append(args, *filter.From)
		argPos++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("date <= $%d", argPos))
		args = append(args, *filter.To)
		argPos++
	}

	if filter.MinDistance != nil {
		conditions = append(conditions, fmt.Sprintf("distance >= $%d", argPos))
		args = append(args, *filter.MinDistance)
		argPos++
	}
	if filter.MaxDistance != nil {
		conditions = append(conditions, fmt.Sprintf("distance <= $%d", argPos))
		args = append(args, *filter.MaxDistance)
		argPos++
	}

	if filter.MinDuration != nil {
		conditions = append(conditions, fmt.Sprintf("duration >= $%d", argPos))
		args = append(args, *filter.MinDuration)
		argPos++
	}
	if filter.MaxDuration != nil {
		conditions = append(conditions, fmt.Sprintf("duration <= $%d", argPos))
		args = append(args, *filter.MaxDuration)
		argPos++
	}

	if filter.MinAvgHR != nil {
		conditions = append(conditions, fmt.Sprintf("avg_hr >= $%d", argPos))
		args = append(args, *filter.MinAvgHR)
		argPos++
	}
	if filter.MaxAvgHR != nil {
		conditions = append(conditions, fmt.Sprintf("avg_hr <= $%d", argPos))
		args = append(args, *filter.MaxAvgHR)
		argPos++
	}

	if filter.MinPace != nil {
		conditions = append(conditions, fmt.Sprintf("pace >= $%d", argPos))
		args = append(args, *filter.MinPace)
		argPos++
	}
	if filter.MaxPace != nil {
		conditions = append(conditions, fmt.Sprintf("pace <= $%d", argPos))
		args = append(args, *filter.MaxPace)
		argPos++
	}

	if filter.MinRPE != nil {
		conditions = append(conditions, fmt.Sprintf("rpe >= $%d", argPos))
		args = append(args, *filter.MinRPE)
		argPos++
	}
	if filter.MaxRPE != nil {
		conditions = append(conditions, fmt.Sprintf("rpe <= $%d", argPos))
		args = append(args, *filter.MaxRPE)
		argPos++
	}

	if filter.HasNotes != nil {
		if *filter.HasNotes {
			conditions = append(conditions, "notes IS NOT NULL AND notes != ''")
		} else {
			conditions = append(conditions, "(notes IS NULL OR notes = '')")
		}
	}

	if filter.HasHRZones != nil {
		if *filter.HasHRZones {
			conditions = append(conditions, "time_in_hr_zone IS NOT NULL AND array_length(time_in_hr_zone, 1) > 0")
		} else {
			conditions = append(conditions, "(time_in_hr_zone IS NULL OR array_length(time_in_hr_zone, 1) = 0)")
		}
	}

	if filter.Shoes != "" {
		conditions = append(conditions, fmt.Sprintf("shoes = $%d", argPos))
		args = append(args, filter.Shoes)
		argPos++
	}

	whereClause := "WHERE user_id = $1"
	if len(conditions) > 0 {
		whereClause += " AND " + strings.Join(conditions, " AND ")
	}

	return whereClause, args, argPos
}

// buildOrderAndPagination строит ORDER BY и LIMIT/OFFSET части.
func (r *workoutRepository) buildOrderAndPagination(filter dto.WorkoutFilter, argPos int) (string, []interface{}) {
	var sb strings.Builder
	args := []interface{}{}

	sortColumn := "date"
	switch filter.SortBy {
	case "distance", "pace", "avg_hr", "calories", "training_load":
		sortColumn = filter.SortBy
	}

	order := "DESC"
	if strings.ToLower(filter.SortOrder) == "asc" {
		order = "ASC"
	}
	sb.WriteString(fmt.Sprintf(" ORDER BY %s %s", sortColumn, order))

	if filter.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT $%d", argPos))
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET $%d", argPos))
		args = append(args, filter.Offset)
	}

	return sb.String(), args
}
