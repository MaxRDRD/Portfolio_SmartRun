package fit

import (
	"bytes"
	"context"
	"fmt"

	"SmartRun/internal/ports/outgoing/importer"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/filedef"
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

	// Берём первую сессию
	session := activity.Sessions[0]

	// Заполняем ActivityData

	ad := &importer.ActivityData{
		StartTime:    session.StartTime,
		Distance:     float64(session.TotalDistance) / 1000.0,
		Duration:     int(session.TotalElapsedTime),
		TypeActivity: session.Sport.String(),
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
		ad.Calories = &cal
	}
	if session.TrainingLoadPeak > 0 {
		trainLoad := float64(session.TrainingLoadPeak)
		ad.TrainingLoad = &trainLoad
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

	return ad, nil
}
