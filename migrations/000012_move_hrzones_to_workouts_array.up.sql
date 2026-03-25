ALTER TABLE workouts
ADD COLUMN IF NOT EXISTS time_in_hr_zone INTEGER[];

UPDATE workouts w
SET time_in_hr_zone = ARRAY[
    COALESCE(h.zone1, 0),
    COALESCE(h.zone2, 0),
    COALESCE(h.zone3, 0),
    COALESCE(h.zone4, 0),
    COALESCE(h.zone5, 0)
]
FROM hrzones h
WHERE h.workout_id = w.id
  AND w.time_in_hr_zone IS NULL;

DROP TABLE IF EXISTS hrzones;
