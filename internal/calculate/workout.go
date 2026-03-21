package calculate

import (
	"SmartRun/internal/model"
	"math"
)

// CalculateDerivedMetrics рассчитывает метрики тренировки
func CalculateDerivedMetrics(workout *model.Workouts, u *model.User) {

	// ---------------------------
	// 1. Pace (сек/км)
	// ---------------------------
	if workout.Distance > 0 && workout.Duration > 0 {
		workout.Pace = float64(workout.Duration) / workout.Distance

	}

	// ---------------------------
	// 2. Calories (ACSM)
	// ---------------------------
	if workout.Distance > 0 && workout.Duration > 0 && u.WeightKg > 0 {
		// speed (м/мин)
		speed := workout.Distance / (float64(workout.Duration) / 60.0)

		// grade (уклон)
		grade := 0.0
		if workout.ElevationGain != nil && workout.Distance > 0 {
			grade = *workout.ElevationGain / workout.Distance
		}

		// ограничиваем уклон
		grade = math.Max(-0.3, math.Min(0.3, grade))

		// VO2 (ACSM)
		vo2 := 0.2*speed + 0.9*speed*grade + 3.5

		// kcal/min
		kcalPerMin := vo2 * u.WeightKg / 1000.0 * 5.0

		totalCalories := kcalPerMin * (float64(workout.Duration) / 60.0)

		calRounded := int(math.Round(totalCalories))
		workout.Calories = &calRounded
	}

	// ---------------------------
	// 3. TRIMP (Banister)
	// ---------------------------
	if workout.AvgHR != nil && u.RestingHR > 0 && u.MaxHR > u.RestingHR {
		hrReserve := float64(*workout.AvgHR - u.RestingHR)
		maxReserve := float64(u.MaxHR - u.RestingHR)

		if maxReserve > 0 {
			hrr := hrReserve / maxReserve
			hrr = math.Max(0, math.Min(1, hrr))

			y := 1.92
			if u.Gender == "female" || u.Gender == "f" {
				y = 1.67
			}

			durationHours := float64(workout.Duration) / 3600.0
			trimp := durationHours * hrr * (0.64 * math.Exp(y*hrr)) * 100

			trimpRounded := math.Round(trimp*10) / 10
			workout.TrainingLoad = &trimpRounded
		}
	}

	// ---------------------------
	// 4. Recovery (нелинейная модель)
	// ---------------------------
	if workout.TrainingLoad != nil {
		load := *workout.TrainingLoad

		// нелинейная зависимость (ближе к EPOC моделям)
		recovery := 6 * math.Log(load+1)

		recovery = math.Max(4, recovery)
		recovery = math.Min(96, recovery)

		rec := int(math.Round(recovery))
		workout.RecoveryTime = &rec
	}

	// ---------------------------
	// 5. Efficiency (pace / HR)
	// ---------------------------
	if workout.Pace > 0 && workout.AvgHR != nil && *workout.AvgHR > 0 {
		eff := workout.Pace / float64(*workout.AvgHR)
		workout.Efficiency = &eff
	}

}

func CalculateTrainingEffect(zones *model.WorkoutHRZones, durationSeconds int) (*float64, *float64) {
	if zones == nil || durationSeconds == 0 {
		return nil, nil
	}

	totalSeconds := float64(durationSeconds)

	// Aerobic Training Effect (в основном Z2 + Z3 + часть Z4)
	aerobicPoints := float64(zones.Zone2Seconds)*1.0 +
		float64(zones.Zone3Seconds)*2.0 +
		float64(zones.Zone4Seconds)*2.8 +
		float64(zones.Zone5Seconds)*1.2

	aerobic := (aerobicPoints / totalSeconds) * 300 // масштабирование до 0-5
	aerobic = math.Max(0.0, math.Min(5.0, aerobic))

	// Anaerobic Training Effect (в основном Z4 + Z5)
	anaerobicPoints := float64(zones.Zone4Seconds)*1.8 +
		float64(zones.Zone5Seconds)*5.0

	anaerobic := (anaerobicPoints / totalSeconds) * 180
	anaerobic = math.Max(0.0, math.Min(5.0, anaerobic))

	return &aerobic, &anaerobic
}

func DeterminePrimaryFocus(aerobic, anaerobic float64, zones *model.WorkoutHRZones) string {
	zonesTotalSecond := zones.Zone1Seconds + zones.Zone2Seconds + zones.Zone3Seconds + zones.Zone4Seconds + zones.Zone5Seconds
	if zones == nil || zonesTotalSecond == 0 {
		// если нет зон — смотрим только на числовой эффект
		if anaerobic > 3.8 {
			return "Anaerobic Power"
		}
		if anaerobic > 2.8 {
			return "Anaerobic"
		}
		if aerobic > 3.8 {
			return "VO2max / Threshold"
		}
		if aerobic > 2.8 {
			return "Aerobic Power"
		}
		return "Aerobic Endurance"
	}

	// ────────────────────────────────────────────────
	// Вариант 1: считаем долю высокоинтенсивной работы
	// ────────────────────────────────────────────────
	highIntensitySeconds := zones.Zone4Seconds + zones.Zone5Seconds
	highIntensityShare := float64(highIntensitySeconds) / float64(zonesTotalSecond)

	// ────────────────────────────────────────────────
	// Вариант 2: взвешенная анаэробная доля (Zone5 весит больше)
	// ────────────────────────────────────────────────
	anaerobicWeighted := float64(zones.Zone5Seconds)*1.0 + float64(zones.Zone4Seconds)*0.55
	anaerobicShare := anaerobicWeighted / float64(zonesTotalSecond)

	// ────────────────────────────────────────────────
	// Логика принятия решения (примеры порогов — подстраивай под себя)
	// ────────────────────────────────────────────────
	switch {
	case anaerobic > 4.0 || (anaerobic > 3.2 && anaerobicShare > 0.18):
		return "Anaerobic Power"

	case anaerobic > 2.8 || (anaerobic > 2.0 && anaerobicShare > 0.10):
		return "Anaerobic"

	case aerobic > 4.0 || (aerobic > 3.5 && highIntensityShare < 0.12):
		return "VO2max / Threshold"

	case aerobic > 3.2:
		return "Aerobic Power"

	default:
		return "Aerobic Endurance"
	}
}
