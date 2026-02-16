CREATE TABLE IF NOT EXISTS metrics(
    id SERIAL PRIMARY KEY,
    workout_id INTEGER NOT NULL,
    pace INTEGER NOT NULL,
    time_run INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_metrics_workouts
        FOREIGN KEY (workout_id)
        REFERENCES workouts(id)
        ON DELETE CASCADE
);