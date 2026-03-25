package postgres

// selectWorkoutColumns возвращает список колонн для SELECT
const selectWorkoutColumns = `
	id, date, distance, duration, pace, type_activity, calories,
	avg_hr, max_hr, elevation_gain, avg_cadence, max_cadence,
	notes, shoes, vo2max_estimate, aerobic_training_effect, anaerobic_training_effect,
	training_load, training_stress_score, intensity_factor, avg_stress, sdrr_hrv, rmssd_hrv,
	COALESCE(time_in_hr_zone, '{}'), recovery_time, rpe, efficiency, primary_training_focus, elevation_loss
`
