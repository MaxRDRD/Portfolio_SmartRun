package calculate

import (
	"SmartRun/internal/model"
	"math"
)

func CalculateDerivedMetrics(workout *model.Workouts, u *model.User) {
	// ====================== ПОДГОТОВКА ======================
	restingHR := u.RestingHR
	if restingHR <= 0 {
		restingHR = 60
	}
	maxHR := u.MaxHR
	if maxHR <= restingHR && workout.MaxHR != nil && *workout.MaxHR > restingHR {
		maxHR = *workout.MaxHR
	}
	if maxHR <= restingHR && u.Age > 0 {
		est := int(math.Round(208.0 - 0.7*float64(u.Age)))
		if est > restingHR {
			maxHR = est
		}
	}

	// ====================== 1. Pace ======================
	if workout.Distance > 0 && workout.Duration > 0 {
		workout.Pace = (float64(workout.Duration) / 60.0) / workout.Distance
	} else {
		workout.Pace = 0
	}

	// ====================== 2. Calories ======================
	if (workout.Calories == nil || *workout.Calories <= 0) && workout.Distance > 0 && workout.Duration > 0 && u.WeightKg > 0 {
		speed := (workout.Distance * 1000.0) / (float64(workout.Duration) / 60.0)
		grade := 0.0
		if workout.ElevationGain != nil {
			// elevation is meters and distance is km; convert distance to meters for slope.
			grade = *workout.ElevationGain / (workout.Distance * 1000.0)
		}
		grade = math.Max(-0.3, math.Min(0.3, grade))

		vo2 := 0.2*speed + 0.9*speed*grade + 3.5
		kcalPerMin := vo2 * u.WeightKg / 1000.0 * 5.0
		totalCalories := kcalPerMin * (float64(workout.Duration) / 60.0)
		calRounded := int(math.Round(totalCalories))
		workout.Calories = &calRounded
	}

	// ====================== 3. Intensity Factor + TSS ======================
	workout.IntensityFactor = nil
	workout.TrainingStressScore = nil
	workout.TrainingLoad = nil

	if workout.Pace > 0 && workout.Duration > 0 {
		ifValue := 0.0

		if u.ThresholdPace > 0 {
			ifValue = u.ThresholdPace / workout.Pace
		} else if workout.AvgHR != nil && maxHR > 0 {
			ifValue = float64(*workout.AvgHR) / float64(maxHR) * 1.08 // чуть выше, чтобы гонки не занижались
		}

		if ifValue > 0 {
			ifValue = math.Max(0.6, math.Min(1.30, ifValue))
			ifRounded := math.Round(ifValue*100) / 100
			workout.IntensityFactor = &ifRounded

			durationHours := float64(workout.Duration) / 3600.0
			tss := durationHours * ifValue * ifValue * 100
			tssRounded := math.Round(tss*10) / 10

			workout.TrainingStressScore = &tssRounded
			workout.TrainingLoad = &tssRounded
		}
	}

	// ====================== 4. Recovery Time (production-safe calibration) ======================
	workout.RecoveryTime = nil
	if workout.TrainingStressScore != nil && workout.AnaerobicTrainingEffect != nil && workout.IntensityFactor != nil {
		durationHours := float64(workout.Duration) / 3600.0
		tss := *workout.TrainingStressScore
		ifValue := *workout.IntensityFactor
		anaerobic := *workout.AnaerobicTrainingEffect

		hrRatio := 0.0
		if workout.AvgHR != nil && maxHR > 0 {
			hrRatio = float64(*workout.AvgHR) / float64(maxHR)
		}

		highIntensityShare := 0.0
		if len(workout.TimeInHrZone) > 0 {
			_, _, _, z4, z5 := firstFiveZones(workout.TimeInHrZone)
			total := 0
			for _, z := range workout.TimeInHrZone {
				total += z
			}
			if total > 0 {
				highIntensityShare = float64(z4+z5) / float64(total)
			}
		}

		recoveryHours := 4.0 + durationHours*4.8
		recoveryHours += math.Max(0, tss-30.0) * 0.22
		recoveryHours += math.Max(0, ifValue-0.88) * 14.0
		recoveryHours += math.Max(0, anaerobic-3.5) * 1.6

		// Short hard races (5K/10K effort) need at least ~1 day even if one signal is noisy.
		hardShortRace := workout.Duration >= 15*60 && workout.Duration <= 40*60 &&
			(ifValue >= 0.95 || hrRatio >= 0.90 || highIntensityShare >= 0.32 || tss >= 70)
		if hardShortRace {
			recoveryHours = math.Max(recoveryHours, 26.0)
		}

		veryHardShortRace := workout.Duration >= 15*60 && workout.Duration <= 40*60 &&
			(ifValue >= 1.02 || hrRatio >= 0.93 || highIntensityShare >= 0.40 || anaerobic >= 3.2)
		if veryHardShortRace {
			recoveryHours = math.Max(recoveryHours, 32.0)
		}

		// Easy/steady sessions should not produce multi-day recovery.
		if ifValue < 0.80 && durationHours <= 1.5 && !hardShortRace {
			recoveryHours = math.Min(recoveryHours, 18.0)
		}

		recoveryHours = math.Max(6.0, math.Min(42.0, recoveryHours))

		recMin := int(math.Round(recoveryHours * 60))
		workout.RecoveryTime = &recMin
	}

	// ====================== 5. Efficiency ======================
	if workout.Pace > 0 && workout.AvgHR != nil && *workout.AvgHR > 0 {
		eff := workout.Pace / float64(*workout.AvgHR)
		workout.Efficiency = &eff
	} else {
		workout.Efficiency = nil
	}

	// ====================== 6. Training Effect ======================
	if aerobic, anaerobic := CalculateTrainingEffect(workout.TimeInHrZone, workout.Duration); aerobic != nil && anaerobic != nil {
		workout.AerobicTrainingEffect = aerobic
		workout.AnaerobicTrainingEffect = anaerobic
		workout.PrimaryTrainingFocus = DeterminePrimaryFocus(*aerobic, *anaerobic, workout.TimeInHrZone)
	} else if workout.IntensityFactor != nil {
		durationHours := float64(workout.Duration) / 3600.0
		ifValue := *workout.IntensityFactor

		aerobic := math.Max(0.0, math.Min(5.0, ifValue*2.2+durationHours*0.7))
		anaerobic := math.Max(0.0, math.Min(5.0, (ifValue-0.85)*9.0+durationHours*0.1))

		aerobic = math.Round(aerobic*10) / 10
		anaerobic = math.Round(anaerobic*10) / 10

		workout.AerobicTrainingEffect = &aerobic
		workout.AnaerobicTrainingEffect = &anaerobic
		workout.PrimaryTrainingFocus = DeterminePrimaryFocus(aerobic, anaerobic, nil)
	} else {
		workout.AerobicTrainingEffect = nil
		workout.AnaerobicTrainingEffect = nil
		workout.PrimaryTrainingFocus = ""
	}
}

// ====================== Training Effect (подкручено) ======================
func CalculateTrainingEffect(zones []int, durationSeconds int) (*float64, *float64) {
	if len(zones) == 0 || durationSeconds == 0 {
		return nil, nil
	}
	_, z2, z3, z4, z5 := firstFiveZones(zones)
	if z2+z3+z4+z5 <= 0 {
		return nil, nil
	}

	totalSeconds := float64(durationSeconds)

	aerobicPoints := float64(z2)*1.0 + float64(z3)*2.3 + float64(z4)*3.2 + float64(z5)*1.8
	aerobic := (aerobicPoints / totalSeconds) * 260
	aerobic = math.Max(0.0, math.Min(5.0, aerobic))

	anaerobicPoints := float64(z4)*2.5 + float64(z5)*7.0
	anaerobic := (anaerobicPoints / totalSeconds) * 145
	anaerobic = math.Max(0.0, math.Min(5.0, anaerobic))

	return &aerobic, &anaerobic
}

func DeterminePrimaryFocus(aerobic, anaerobic float64, zones []int) string {
	if len(zones) == 0 {
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
	z1, z2, z3, z4, z5 := firstFiveZones(zones)
	zonesTotalSecond := z1 + z2 + z3 + z4 + z5
	if zonesTotalSecond == 0 {
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
	highIntensitySeconds := z4 + z5
	highIntensityShare := float64(highIntensitySeconds) / float64(zonesTotalSecond)

	// ────────────────────────────────────────────────
	// Вариант 2: взвешенная анаэробная доля (Zone5 весит больше)
	// ────────────────────────────────────────────────
	anaerobicWeighted := float64(z5)*1.0 + float64(z4)*0.55
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

func firstFiveZones(zones []int) (int, int, int, int, int) {
	var z [5]int
	for i := 0; i < len(zones) && i < 5; i++ {
		z[i] = zones[i]
	}
	return z[0], z[1], z[2], z[3], z[4]
}
