CREATE TABLE IF NOT EXISTS hrzones (
    workout_id INTEGER PRIMARY KEY REFERENCES workouts(id) ON DELETE CASCADE,
    zone1      INTEGER,
    zone2      INTEGER,
    zone3      INTEGER,
    zone4      INTEGER,
    zone5      INTEGER
);

INSERT INTO hrzones (workout_id, zone1, zone2, zone3, zone4, zone5)
SELECT
    w.id,
    COALESCE(w.time_in_hr_zone[1], 0),
    COALESCE(w.time_in_hr_zone[2], 0),
    COALESCE(w.time_in_hr_zone[3], 0),
    COALESCE(w.time_in_hr_zone[4], 0),
    COALESCE(w.time_in_hr_zone[5], 0)
FROM workouts w
WHERE w.time_in_hr_zone IS NOT NULL
ON CONFLICT (workout_id) DO UPDATE SET
    zone1 = EXCLUDED.zone1,
    zone2 = EXCLUDED.zone2,
    zone3 = EXCLUDED.zone3,
    zone4 = EXCLUDED.zone4,
    zone5 = EXCLUDED.zone5;

ALTER TABLE workouts
DROP COLUMN IF EXISTS time_in_hr_zone;
