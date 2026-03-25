package calculate

import (
	"SmartRun/internal/model"
	"math"
	"sort"
	"time"
)

func CalculateReadiness(workouts []model.Workouts, previousMetrics *model.DailyMetric) *model.DailyMetric {
	thresholdPace := estimateThresholdPaceMinKm(workouts)
	atl := rollingStress(workouts, 7, thresholdPace)
	ctl := rollingStress(workouts, 42, thresholdPace)

	// Легкая инерция между днями, чтобы метрика не прыгала слишком резко.
	if previousMetrics != nil {
		atl = previousMetrics.ATL*0.5 + atl*0.5
		ctl = previousMetrics.CTL*0.8 + ctl*0.2
	}

	tsb := ctl - atl

	var fatigueScore float64
	if ctl > 0.1 {
		fatigueScore = (atl / ctl) * 100.0
	} else {
		fatigueScore = 0
	}
	fatigueScore = clampFloat(fatigueScore, 0, 100)

	readinessScore := clampFloat(100-fatigueScore, 0, 100)
	readinessScore += recoveryBonusFromLatestWorkout(workouts)
	readinessScore += multiFactorRecoveryAdjustment(workouts, previousMetrics)
	readinessScore = clampFloat(readinessScore, 0, 100)

	recommendation := recommendationByTSB(tsb)

	return &model.DailyMetric{
		Date:           time.Now().UTC(),
		ATL:            atl,
		CTL:            ctl,
		TSB:            tsb,
		FatigueScore:   int(math.Round(fatigueScore)),
		ReadinessScore: int(math.Round(readinessScore)),
		Recommendation: recommendation,
		UpdatedAt:      time.Now().UTC(),
	}
}

func rollingStress(workouts []model.Workouts, days int, thresholdPaceMinKm float64) float64 {
	if days <= 0 {
		return 0
	}
	limit := days
	if len(workouts) < limit {
		limit = len(workouts)
	}
	if limit == 0 {
		return 0
	}

	sum := 0.0
	for i := 0; i < limit; i++ {
		sum += workoutStressScore(workouts[i], thresholdPaceMinKm)
	}
	return sum / float64(limit)
}

func workoutStressScore(w model.Workouts, thresholdPaceMinKm float64) float64 {
	if w.TrainingLoad != nil && *w.TrainingLoad > 0 && *w.TrainingLoad < 400 {
		return *w.TrainingLoad
	}

	if w.Duration <= 0 {
		return 0
	}

	pace := w.Pace
	if pace <= 0 && w.Distance > 0 {
		pace = (float64(w.Duration) / 60.0) / w.Distance
	}
	if pace <= 0 || thresholdPaceMinKm <= 0 {
		return 0
	}

	// Для pace в мин/км интенсивность >1 означает темп быстрее порога.
	ifactor := thresholdPaceMinKm / pace
	ifactor = clampFloat(ifactor, 0.6, 1.6)

	durationHours := float64(w.Duration) / 3600.0
	tss := durationHours * ifactor * ifactor * 100.0

	if w.RPE != nil {
		rpeAdj := 1.0 + (float64(*w.RPE)-5.0)*0.05
		tss *= clampFloat(rpeAdj, 0.8, 1.2)
	}

	return clampFloat(tss, 0, 350)
}

func estimateThresholdPaceMinKm(workouts []model.Workouts) float64 {
	candidates := make([]float64, 0, len(workouts))

	for _, w := range workouts {
		if w.Duration < 20*60 || w.Duration > 90*60 {
			continue
		}
		pace := w.Pace
		if pace <= 0 && w.Distance > 0 {
			pace = (float64(w.Duration) / 60.0) / w.Distance
		}
		if pace <= 0 {
			continue
		}
		candidates = append(candidates, pace)
	}

	if len(candidates) == 0 {
		for _, w := range workouts {
			if w.Pace > 0 {
				candidates = append(candidates, w.Pace)
			}
		}
	}

	if len(candidates) == 0 {
		return 5.3 // безопасный дефолт для взрослого любителя
	}

	sort.Float64s(candidates)
	fastCount := int(math.Ceil(float64(len(candidates)) * 0.2))
	if fastCount < 1 {
		fastCount = 1
	}

	sum := 0.0
	for i := 0; i < fastCount; i++ {
		sum += candidates[i]
	}
	fastMean := sum / float64(fastCount)

	// Порог обычно чуть медленнее самых быстрых устойчивых сессий.
	return clampFloat(fastMean+0.25, 3.0, 8.0)
}

func recommendationByTSB(tsb float64) string {
	switch {
	case tsb >= 10:
		return "Высокая готовность: можно делать качественную работу"
	case tsb >= -10:
		return "Нормальная готовность: поддерживающая тренировка"
	default:
		return "Низкая готовность: лучше восстановление/легкий бег"
	}
}

func recoveryBonusFromLatestWorkout(workouts []model.Workouts) float64 {
	if len(workouts) == 0 {
		return 0
	}
	latest := workouts[0]
	if latest.RPE == nil {
		return 0
	}

	if *latest.RPE <= 4 {
		return 5
	}
	if *latest.RPE >= 8 {
		return -8
	}
	return 0
}

func multiFactorRecoveryAdjustment(workouts []model.Workouts, previousMetrics *model.DailyMetric) float64 {
	if len(workouts) == 0 {
		return 0
	}

	latest := workouts[0]
	adj := 0.0

	// Stress contribution.
	if latest.AvgStress != nil {
		s := float64(*latest.AvgStress)
		switch {
		case s >= 75:
			adj -= 8
		case s >= 60:
			adj -= 4
		case s <= 25:
			adj += 3
		}
	}

	// HRV contribution (using relative deviation from recent baseline when possible).
	rmssdBaseline := recentMedianHRV(workouts, true)
	if latest.RmssdHrv != nil && rmssdBaseline > 0 {
		ratio := float64(*latest.RmssdHrv) / rmssdBaseline
		switch {
		case ratio < 0.85:
			adj -= 6
		case ratio < 0.95:
			adj -= 3
		case ratio > 1.10:
			adj += 3
		}
	}

	sdrrBaseline := recentMedianHRV(workouts, false)
	if latest.SdrrHrv != nil && sdrrBaseline > 0 {
		ratio := float64(*latest.SdrrHrv) / sdrrBaseline
		switch {
		case ratio < 0.85:
			adj -= 4
		case ratio > 1.10:
			adj += 2
		}
	}

	// Optional sleep signal from previous daily metrics.
	if previousMetrics != nil && previousMetrics.SleepScore > 0 {
		sleep := float64(previousMetrics.SleepScore)
		switch {
		case sleep < 50:
			adj -= 6
		case sleep < 65:
			adj -= 3
		case sleep >= 85:
			adj += 4
		case sleep >= 75:
			adj += 2
		}
	}

	return clampFloat(adj, -20, 15)
}

func recentMedianHRV(workouts []model.Workouts, useRMSSD bool) float64 {
	values := make([]float64, 0, 14)
	for i := 0; i < len(workouts) && i < 14; i++ {
		w := workouts[i]
		if useRMSSD {
			if w.RmssdHrv != nil && *w.RmssdHrv > 0 {
				values = append(values, float64(*w.RmssdHrv))
			}
		} else {
			if w.SdrrHrv != nil && *w.SdrrHrv > 0 {
				values = append(values, float64(*w.SdrrHrv))
			}
		}
	}

	if len(values) == 0 {
		return 0
	}

	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2
	}
	return values[mid]
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
