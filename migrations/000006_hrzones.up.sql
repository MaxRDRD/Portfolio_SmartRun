CREATE TABLE hrzones (
    workout_id INTEGER PRIMARY KEY REFERENCES workouts(id) ON DELETE CASCADE,
    zone1      INTEGER,
    zone2      INTEGER,
    zone3      INTEGER,
    zone4      INTEGER,
    zone5      INTEGER
);