ALTER TABLE workouts
DROP COLUMN IF EXISTS rmssd_hrv,
DROP COLUMN IF EXISTS sdrr_hrv,
DROP COLUMN IF EXISTS avg_stress,
DROP COLUMN IF EXISTS intensity_factor,
DROP COLUMN IF EXISTS training_stress_score;