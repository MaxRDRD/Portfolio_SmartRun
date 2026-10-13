package validate

import (
	"SmartRun/internal/model"
	"fmt"
	"strings"
)

// CheckForAnomalies проверяет тренировку на наличие аномальных данных
// (например, сверхчеловеческого темпа или невозможной частоты пульса).
func CheckForAnomalies(w *model.Workouts) (bool, string) {
	var reasons []string

	// Проверка темпа (минут на километр)
	// < 2,0 мин/км — быстрее мирового рекорда в спринте на длинной дистанции
	if w.Pace > 0 && w.Pace < 2.0 {
		reasons = append(reasons, fmt.Sprintf("Pace is unusually fast: %.2f min/km", w.Pace))
	}
	// > 20,0 мин/км — это, по сути, отсутствие движения
	if w.Pace > 20.0 {
		reasons = append(reasons, fmt.Sprintf("Pace is unusually slow: %.2f min/km", w.Pace))
	}

	// Max HR check
	if w.MaxHR != nil && *w.MaxHR > 220 {
		reasons = append(reasons, fmt.Sprintf("Max HR is anomalously high: %d", *w.MaxHR))
	}

	// Avg HR check
	if w.AvgHR != nil && *w.AvgHR < 30 {
		reasons = append(reasons, fmt.Sprintf("Avg HR is anomalously low: %d", *w.AvgHR))
	}

	// Negative distance or duration
	if w.Distance < 0 {
		reasons = append(reasons, fmt.Sprintf("Distance is negative: %.2f", w.Distance))
	}
	if w.Duration < 0 {
		reasons = append(reasons, fmt.Sprintf("Duration is negative: %d", w.Duration))
	}

	if len(reasons) > 0 {
		return true, strings.Join(reasons, "; ")
	}

	return false, ""
}
