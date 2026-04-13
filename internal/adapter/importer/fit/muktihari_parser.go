package fit

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"SmartRun/internal/ports/outgoing/importer"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
)

type MuktihariFitParser struct{}

func NewMuktihariFitParser() importer.FitParser {
	return &MuktihariFitParser{}
}

func (p *MuktihariFitParser) Parse(ctx context.Context, data []byte) (*importer.ActivityData, error) {
	dec := decoder.New(bytes.NewReader(data))

	// Декодируем все сообщения в сыром виде
	fit, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode FIT file: %w", err)
	}

	activity := filedef.NewActivity(fit.Messages...)

	if len(activity.Sessions) == 0 {
		return nil, fmt.Errorf("no sessions found in FIT file")
	}
	// Берём первую сессию
	session := activity.Sessions[0]

	// Заполняем ActivityData

	// FIT: TotalTimerTime Scale=1000, Units=s
	durationSec := int(session.TotalTimerTime / 1000)
	if durationSec <= 0 {
		return nil, fmt.Errorf("invalid duration: %d ms", session.TotalTimerTime)
	}

	// FIT: TotalDistance Scale=100, Units=m -> km = raw / 100 / 1000 = raw / 100000
	distanceKm := float64(session.TotalDistance) / 100000.0
	if distanceKm < 0 {
		return nil, fmt.Errorf("invalid distance: %.2f km", distanceKm)
	}

	ad := &importer.ActivityData{
		StartTime:    session.StartTime,
		Distance:     distanceKm,
		Duration:     durationSec,
		TypeActivity: normalizeActivityType(session.Sport.String()),
	}

	// Опциональные поля
	if session.AvgHeartRate > 0 {
		avg := int(session.AvgHeartRate)
		ad.AvgHR = &avg
	}
	if session.MaxHeartRate > 0 {
		max := int(session.MaxHeartRate)
		ad.MaxHR = &max
	}
	if session.AvgCadence > 0 {
		avgCad := int(session.AvgCadence)
		ad.AvgCadence = &avgCad
	}
	if session.MaxCadence > 0 {
		maxCad := int(session.MaxCadence)
		ad.MaxCadence = &maxCad
	}
	if session.TotalAscent > 0 {
		gain := float64(session.TotalAscent)
		ad.ElevationGain = &gain
	}
	if session.TotalDescent > 0 {
		loss := float64(session.TotalDescent)
		ad.ElevationLoss = &loss
	}
	if session.TotalCalories > 0 {
		cal := int(session.TotalCalories)
		// Validate calories: run roughly burns 60-100 kcal/km depending on weight
		// For distance in km, expect max ~150 kcal/km as a sanity check
		maxExpectedCalories := int(distanceKm * 150)
		if cal <= maxExpectedCalories {
			// Only use FIT calories if they're realistic; otherwise skip and let service layer calculate
			ad.Calories = &cal
		}
		// If calories are too high, just skip them - service will auto-calculate
	}
	if session.TrainingLoadPeak > 0 {
		trainLoad := float64(session.TrainingLoadPeak)
		ad.TrainingLoad = &trainLoad
	}
	if session.TrainingStressScore > 0 {
		tss := float64(session.TrainingStressScore) / 10.0
		ad.TrainingStressScore = &tss
	}
	if session.IntensityFactor > 0 {
		ifVal := float64(session.IntensityFactor) / 1000.0
		ad.IntensityFactor = &ifVal
	}
	if session.TotalTrainingEffect != 0 {
		te := float64(session.TotalTrainingEffect) / 10.0
		ad.AerobicTrainingEffect = &te
	}
	if session.TotalAnaerobicTrainingEffect != 0 {
		te := float64(session.TotalAnaerobicTrainingEffect) / 10.0
		ad.AnaerobicTrainingEffect = &te
	}
	if session.WorkoutRpe != 0 {
		rpe := int(session.WorkoutRpe) / 10 // шкала 0-100 → 0-10
		ad.RPE = &rpe
	}
	if session.AvgStress > 0 {
		avgStress := int(session.AvgStress)
		ad.AvgStress = &avgStress
	}
	if session.SdrrHrv > 0 {
		sdrr := int(session.SdrrHrv)
		ad.SdrrHrv = &sdrr
	}
	if session.RmssdHrv > 0 {
		rmssd := int(session.RmssdHrv)
		ad.RmssdHrv = &rmssd
	}

	ad.TimeInHrZone = extractHRZones(activity.Records, activity.TimeInZones, session.TimeInHrZone, durationSec, session.MaxHeartRate)

	return ad, nil
}

func extractHRZones(records []*mesgdef.Record, timeInZones []*mesgdef.TimeInZone, sessionZones []uint32, durationSec int, maxHR uint8) []int {
	if zones := extractHRZonesFromTimeInZones(timeInZones); len(zones) > 0 {
		return zones
	}

	if len(sessionZones) > 0 {
		zones := make([]int, 0, len(sessionZones))
		for _, z := range sessionZones {
			zones = append(zones, normalizeZoneSeconds(z))
		}
		return zones
	}

	if len(records) == 0 {
		return nil
	}

	effectiveMaxHR := int(maxHR)
	if effectiveMaxHR <= 0 {
		for _, rec := range records {
			hr := int(rec.HeartRate)
			if hr > effectiveMaxHR {
				effectiveMaxHR = hr
			}
		}
	}
	if effectiveMaxHR <= 0 {
		return nil
	}

	zones := make([]int, 5)
	var prevTime time.Time
	var prevHR int
	havePrev := false

	for _, rec := range records {
		hr := int(rec.HeartRate)
		if hr <= 0 || rec.Timestamp.IsZero() {
			continue
		}

		if havePrev {
			delta := int(rec.Timestamp.Sub(prevTime).Seconds())
			if delta <= 0 {
				delta = 1
			}
			if delta > 10 {
				delta = 1
			}
			idx := detectHRZone(prevHR, effectiveMaxHR)
			zones[idx] += delta
		}

		prevTime = rec.Timestamp
		prevHR = hr
		havePrev = true
	}

	if !havePrev {
		return nil
	}

	total := 0
	for _, z := range zones {
		total += z
	}
	if total <= 0 {
		return nil
	}

	if durationSec > 0 && total > durationSec {
		scale := float64(durationSec) / float64(total)
		for i := range zones {
			zones[i] = int(math.Round(float64(zones[i]) * scale))
		}
	}

	return zones
}

func extractHRZonesFromTimeInZones(items []*mesgdef.TimeInZone) []int {
	if len(items) == 0 {
		return nil
	}

	// Берём последнюю запись с заполненным TimeInHrZone (обычно самая актуальная summary)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item == nil || len(item.TimeInHrZone) == 0 {
			continue
		}

		zones := make([]int, 0, len(item.TimeInHrZone))
		for _, z := range item.TimeInHrZone {
			zones = append(zones, normalizeZoneSeconds(z))
		}
		return zones
	}

	return nil
}

func normalizeZoneSeconds(raw uint32) int {
	v := int(raw)
	if v <= 0 {
		return 0
	}
	// FIT TimeInHrZone Scale=1000, Units=s
	return int(math.Round(float64(raw) / 1000.0))
}

func detectHRZone(hr, maxHR int) int {
	if maxHR <= 0 {
		return 0
	}
	ratio := float64(hr) / float64(maxHR)
	switch {
	case ratio < 0.60:
		return 0
	case ratio < 0.70:
		return 1
	case ratio < 0.80:
		return 2
	case ratio < 0.90:
		return 3
	default:
		return 4
	}
}

// normalizeActivityType преобразует тип активности из FIT в стандартный формат
func normalizeActivityType(sport string) string {
	if sport == "" {
		return "run"
	}
	// Преобразуем в нижний регистр для унификации
	lower := strings.ToLower(sport)

	// Маппируем известные типы
	switch lower {
	case "running", "run":
		return "run"
	case "cycling", "bike", "cycle":
		return "cycling"
	case "swimming", "swim":
		return "swimming"
	case "walking", "walk":
		return "walk"
	default:
		// Если тип неизвестен, возвращаем его как есть в нижнем регистре
		return lower
	}
}
