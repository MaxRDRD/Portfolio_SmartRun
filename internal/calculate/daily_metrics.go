package calculate

import (
	"SmartRun/internal/model"
	"math"
	"time"
)

// CalculateDailyMetrics рассчитывает дневную метрику на основе тренировок дня и истории.
// workouts: отсортированы по дате (новые первые), обычно получены GetAllByUserID с фильтром по дате
// previousMetrics: метрика предыдущего дня для инерции (может быть nil)
func CalculateDailyMetrics(workouts []model.Workouts, previousMetrics *model.DailyMetric) *model.DailyMetric {
	readiness := CalculateReadiness(workouts, previousMetrics)
	now := time.Now().UTC()
	metricDate := normalizeDateUTC(now)

	if len(workouts) > 0 {
		metricDate = normalizeDateUTC(workouts[0].Date)
	}

	dailyMetric := &model.DailyMetric{
		Date:           metricDate,
		ATL:            readiness.ATL,
		CTL:            readiness.CTL,
		TSB:            readiness.TSB,
		FatigueScore:   readiness.FatigueScore,
		ReadinessScore: readiness.ReadinessScore,
		Recommendation: readiness.Recommendation,
		UpdatedAt:      now,
	}

	// Берем UserID и неизменные данные из предыдущего дня (сон, например)
	if previousMetrics != nil {
		dailyMetric.UserID = previousMetrics.UserID
		dailyMetric.SleepScore = previousMetrics.SleepScore
		// BodyBatteryAvg может быть переопределен ниже
		if previousMetrics.BodyBatteryAvg > 0 && len(workouts) == 0 {
			dailyMetric.BodyBatteryAvg = previousMetrics.BodyBatteryAvg
		}
	}

	if len(workouts) == 0 {
		// Нет тренировок - нет streak, нет monotony/strain
		dailyMetric.StreakDays = 0
		dailyMetric.Monotony = 0
		dailyMetric.Strain = nil
		dailyMetric.TotalCalories = 0
		dailyMetric.StressAvg = 0

		if dailyMetric.BodyBatteryAvg == 0 {
			dailyMetric.BodyBatteryAvg = clampFloat(100-float64(dailyMetric.FatigueScore), 0, 100)
		}
		return dailyMetric
	}

	// UserID и дневные метрики берутся из тренировок сегодня
	dailyMetric.UserID = workouts[0].UserID
	dailyMetric.TotalCalories = sumCaloriesByDate(workouts, metricDate)
	dailyMetric.StressAvg = avgStressByDate(workouts, metricDate)
	dailyMetric.Monotony, dailyMetric.Strain = calcMonotonyAndStrain(workouts, metricDate)

	// Streak обновляется инкрементально от предыдущего дня.
	todayHasWorkout := len(workouts) > 0 && normalizeDateUTC(workouts[0].Date).Equal(metricDate)
	dailyMetric.StreakDays = calcStreakDaysIncremental(todayHasWorkout, previousMetrics)

	// Если нет внешних данных батареи, рассчитываем из усталости
	if dailyMetric.BodyBatteryAvg == 0 {
		dailyMetric.BodyBatteryAvg = clampFloat(100-float64(dailyMetric.FatigueScore), 0, 100)
	}

	return dailyMetric
}

func normalizeDateUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// sumCaloriesByDate суммирует калории за конкретный день из выборки тренировок.
// Early exit при нахождении всех тренировок дня (оптимизация).
func sumCaloriesByDate(workouts []model.Workouts, targetDate time.Time) int {
	total := 0
	for _, w := range workouts {
		// Как только дата станет старше targetDate - выходим (сортировка по новым первым)
		if normalizeDateUTC(w.Date).Before(targetDate) {
			break
		}
		if normalizeDateUTC(w.Date).Equal(targetDate) && w.Calories != nil {
			total += *w.Calories
		}
	}
	return total
}

func avgStressByDate(workouts []model.Workouts, targetDate time.Time) int {
	sum := 0
	count := 0
	for _, w := range workouts {
		if normalizeDateUTC(w.Date).Equal(targetDate) && w.AvgStress != nil {
			sum += *w.AvgStress
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return int(math.Round(float64(sum) / float64(count)))
}

// calcMonotonyAndStrain рассчитывает монотонность нагрузок и общую нагрузку за неделю.
// Monotony = средняя нагрузка / стандартное отклонение (чем выше - тем монотоннее, min=1, max~10).
// Strain = недельная нагрузка * monotony (комбинированный показатель).
// Дни без тренировок считаются как 0 нагрузка (это часть вариативности).
func calcMonotonyAndStrain(workouts []model.Workouts, anchorDate time.Time) (float64, *float64) {
	thresholdPace := estimateThresholdPaceMinKm(workouts)

	// Берем данные за последние 7 дней (включая anchorDate)
	loadsByDate := make(map[time.Time]float64)
	for i := 6; i >= 0; i-- {
		day := anchorDate.AddDate(0, 0, -i)
		loadsByDate[day] = 0 // Инициализируем нулем (день без тренировок)
	}

	// Заполняем имеющиеся нагрузки
	for _, w := range workouts {
		day := normalizeDateUTC(w.Date)
		// Игнорируем тренировки вне 7-дневного окна
		if day.Before(anchorDate.AddDate(0, 0, -6)) || day.After(anchorDate) {
			continue
		}

		load := 0.0
		if w.TrainingLoad != nil && *w.TrainingLoad > 0 {
			load = *w.TrainingLoad
		} else {
			load = workoutStressScore(w, thresholdPace)
		}
		loadsByDate[day] += load
	}

	// Собираем в порядке недели для вычисления статистики
	loads := make([]float64, 0, 7)
	for i := 6; i >= 0; i-- {
		day := anchorDate.AddDate(0, 0, -i)
		loads = append(loads, loadsByDate[day])
	}

	weeklyLoad := 0.0
	for _, v := range loads {
		weeklyLoad += v
	}

	// Если за неделю вообще не было нагрузки - нет чего считать
	if weeklyLoad == 0 {
		return 0, nil
	}

	mean := weeklyLoad / float64(len(loads))
	variance := 0.0
	for _, v := range loads {
		d := v - mean
		variance += d * d
	}
	stdDev := math.Sqrt(variance / float64(len(loads)))

	// Monotony = mean / stdDev
	// Если stdDev близко к нулю (все дни одинаковые) - высокая монотонность
	monotony := 0.0
	if stdDev > 0.001 {
		monotony = mean / stdDev
	} else if mean > 0.001 {
		// Если вариативность минимальна, но нагрузка есть - очень монотонно
		monotony = 10
	}
	monotony = clampFloat(monotony, 0, 10)

	strain := weeklyLoad * monotony
	strain = math.Round(strain*100) / 100

	return math.Round(monotony*100) / 100, &strain
}

func calcStreakDaysIncremental(todayHasWorkout bool, previousMetric *model.DailyMetric) int {
	if previousMetric == nil {
		// Первый день метрики
		if todayHasWorkout {
			return 1
		}
		return 0
	}

	if todayHasWorkout {
		return previousMetric.StreakDays + 1 // продолжаем streak
	}
	return 0 // reset streak
}
