package model

import "time"

type Metrics struct {
	ID            int64
	UserID        int64
	TotalWorkouts int
	TotalDistance float64
	TotalDuration int
	AvgPace       float64
	From          string
	To            string
	TotalCalories int64
}

type DailyMetric struct {
	ID             int64
	UserID         int64
	Date           time.Time
	CTL            float64
	ATL            float64
	TSB            float64
	FatigueScore   int
	ReadinessScore int
	BodyBatteryAvg float64
	Steps          int
	TotalCalories  int
	SleepScore     int
	StressAvg      int
	Recommendation string
	StreakDays     int
	Monotony       float64  // показатель вариативности тренировок, чем выше - тем менее разнообразные тренировки
	Strain         *float64 // показатель общей нагрузки, учитывающий и объем, и интенсивность
	UpdatedAt      time.Time
}
