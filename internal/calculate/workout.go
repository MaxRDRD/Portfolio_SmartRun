package calculate

import (
	"SmartRun/internal/model"
	"math"
)

// CalculateDerivedMetrics выполняет последовательный расчет всех производных метрик тренировки
func CalculateDerivedMetrics(workout *model.Workouts, u *model.User) {
	if workout == nil {
		return
	}

	maxHR := resolveMaxHR(workout, u)

	workout.Pace = calculatePace(workout.Distance, workout.Duration)
	workout.Calories = calculateCalories(workout, u)

	workout.IntensityFactor, workout.TrainingStressScore = calculateIntensityAndTSS(workout, u, maxHR)
	workout.TrainingLoad = workout.TrainingStressScore

	workout.AerobicTrainingEffect, workout.AnaerobicTrainingEffect, workout.PrimaryTrainingFocus =
		calculateTrainingEffectAndFocus(workout)

	workout.RecoveryTime = calculateRecoveryTime(workout, maxHR)
	workout.Efficiency = calculateEfficiency(workout.Pace, workout.AvgHR)
}

// resolveMaxHR определяет максимальный пульс пользователя или тренировки (с оценкой по возрасту Танаки)
func resolveMaxHR(workout *model.Workouts, u *model.User) int {
	if u == nil {
		if workout != nil && workout.MaxHR != nil {
			return *workout.MaxHR
		}
		return 0
	}
	restingHR := u.RestingHR
	if restingHR <= 0 {
		restingHR = 60
	}
	maxHR := u.MaxHR
	if maxHR <= restingHR && workout != nil && workout.MaxHR != nil && *workout.MaxHR > restingHR {
		maxHR = *workout.MaxHR
	}
	if maxHR <= restingHR && u.Age > 0 {
		est := int(math.Round(208.0 - 0.7*float64(u.Age)))
		if est > restingHR {
			maxHR = est
		}
	}
	return maxHR
}

// calculatePace вычисляет темп в мин/км
func calculatePace(distanceKm float64, durationSec int) float64 {
	if distanceKm > 0 && durationSec > 0 {
		return (float64(durationSec) / 60.0) / distanceKm
	}
	return 0
}

// calculateCalories рассчитывает сожженные калории по формуле ACSM с учетом уклона
func calculateCalories(workout *model.Workouts, u *model.User) *int {
	if workout == nil || u == nil {
		return nil
	}
	if workout.Calories != nil && *workout.Calories > 0 {
		return workout.Calories
	}
	if workout.Distance <= 0 || workout.Duration <= 0 || u.WeightKg <= 0 {
		return nil
	}

	speed := (workout.Distance * 1000.0) / (float64(workout.Duration) / 60.0)
	grade := 0.0
	if workout.ElevationGain != nil {
		grade = *workout.ElevationGain / (workout.Distance * 1000.0)
	}
	grade = math.Max(-0.3, math.Min(0.3, grade))

	vo2 := 0.2*speed + 0.9*speed*grade + 3.5
	kcalPerMin := vo2 * u.WeightKg / 1000.0 * 5.0
	totalCalories := kcalPerMin * (float64(workout.Duration) / 60.0)
	calRounded := int(math.Round(totalCalories))
	return &calRounded
}

// calculateIntensityAndTSS вычисляет фактор интенсивности (IF) и тренировочный стресс (TSS)
func calculateIntensityAndTSS(workout *model.Workouts, u *model.User, maxHR int) (*float64, *float64) {
	if workout == nil || workout.Duration <= 0 {
		return nil, nil
	}

	durationHours := float64(workout.Duration) / 3600.0
	ifValue := 0.0

	thresholdPace := 0.0
	if u != nil {
		thresholdPace = u.ThresholdPace
	}

	if workout.Pace > 0 {
		if thresholdPace > 0 {
			ifValue = thresholdPace / workout.Pace
		} else if workout.AvgHR != nil && maxHR > 0 {
			ifValue = float64(*workout.AvgHR) / float64(maxHR) * 1.08
		}
	}

	if ifValue > 0 {
		ifValue = math.Max(0.6, math.Min(1.30, ifValue))
		ifRounded := math.Round(ifValue*100) / 100

		tss := durationHours * ifValue * ifValue * 100
		if workout.RPE != nil && *workout.RPE > 0 {
			rpeTSS := durationHours * float64(*workout.RPE) * 10.0
			tss = (tss + rpeTSS) / 2.0
		}
		tssRounded := math.Round(tss*10) / 10
		return &ifRounded, &tssRounded
	}

	if workout.RPE != nil && *workout.RPE > 0 {
		rpeTSS := durationHours * float64(*workout.RPE) * 10.0
		tssRounded := math.Round(rpeTSS*10) / 10
		return nil, &tssRounded
	}

	return nil, nil
}

// calculateTrainingEffectAndFocus вычисляет тренировочный эффект и первичный фокус
func calculateTrainingEffectAndFocus(workout *model.Workouts) (*float64, *float64, string) {
	if workout == nil {
		return nil, nil, ""
	}

	if aerobic, anaerobic := CalculateTrainingEffect(workout.TimeInHrZone, workout.Duration); aerobic != nil && anaerobic != nil {
		focus := DeterminePrimaryFocus(*aerobic, *anaerobic, workout.TimeInHrZone)
		return aerobic, anaerobic, focus
	}

	if workout.IntensityFactor != nil {
		durationHours := float64(workout.Duration) / 3600.0
		ifValue := *workout.IntensityFactor

		aerobic := math.Max(0.0, math.Min(5.0, ifValue*2.2+durationHours*0.7))
		anaerobic := math.Max(0.0, math.Min(5.0, (ifValue-0.85)*9.0+durationHours*0.1))

		aerobic = math.Round(aerobic*10) / 10
		anaerobic = math.Round(anaerobic*10) / 10

		focus := DeterminePrimaryFocus(aerobic, anaerobic, nil)
		return &aerobic, &anaerobic, focus
	}

	return nil, nil, ""
}

// calculateRecoveryTime вычисляет требуемое время восстановления в минутах
func calculateRecoveryTime(workout *model.Workouts, maxHR int) *int {
	if workout == nil || workout.TrainingStressScore == nil || workout.AnaerobicTrainingEffect == nil || workout.IntensityFactor == nil {
		return nil
	}

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

	if ifValue < 0.80 && durationHours <= 1.5 && !hardShortRace {
		recoveryHours = math.Min(recoveryHours, 18.0)
	}

	recoveryHours = math.Max(6.0, math.Min(42.0, recoveryHours))
	recMin := int(math.Round(recoveryHours * 60))
	return &recMin
}

// calculateEfficiency вычисляет беговую эффективность (темп / ЧСС)
func calculateEfficiency(pace float64, avgHR *int) *float64 {
	if pace > 0 && avgHR != nil && *avgHR > 0 {
		eff := pace / float64(*avgHR)
		return &eff
	}
	return nil
}

// CalculateTrainingEffect рассчитывает аэробный и анаэробный тренировочный эффект по зонам пульса
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
	aerobic := (aerobicPoints / totalSeconds) * 2.6
	aerobic = math.Max(0.0, math.Min(5.0, aerobic))

	anaerobicPoints := float64(z4)*2.5 + float64(z5)*7.0
	anaerobic := (anaerobicPoints / totalSeconds) * 1.45
	anaerobic = math.Max(0.0, math.Min(5.0, anaerobic))

	return &aerobic, &anaerobic
}

// DeterminePrimaryFocus определяет тренировочный фокус (Aerobic Endurance, VO2max, Anaerobic Power и др.)
func DeterminePrimaryFocus(aerobic, anaerobic float64, zones []int) string {
	if len(zones) == 0 {
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

	highIntensitySeconds := z4 + z5
	highIntensityShare := float64(highIntensitySeconds) / float64(zonesTotalSecond)

	anaerobicWeighted := float64(z5)*1.0 + float64(z4)*0.55
	anaerobicShare := anaerobicWeighted / float64(zonesTotalSecond)

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
