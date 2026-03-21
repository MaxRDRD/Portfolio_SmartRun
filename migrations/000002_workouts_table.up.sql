CREATE TABLE IF NOT EXISTS workouts(
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    distance DECIMAL(7,2) NOT NULL,
    duration INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    date TIMESTAMP NOT NULL DEFAULT NOW(),
    pace DECIMAL(7,2) NOT NULL,
    type_activity VARCHAR(100) NOT NULL,
    calories INTEGER,
    avg_hr         INTEGER,
    max_hr         INTEGER,
    avg_cadence    INTEGER,
    max_cadence    INTEGER,
    elevation_gain DOUBLE PRECISION,
    elevation_loss DOUBLE PRECISION,
    vo2max_estimate DOUBLE PRECISION,
    aerobic_training_effect DOUBLE PRECISION,
    anaerobic_training_effect DOUBLE PRECISION,
    primary_training_focus TEXT,
    training_load DOUBLE PRECISION,
    recovery_time INTEGER,
    rpe            SMALLINT,
    notes          TEXT,
    shoes          TEXT, 
    efficiency DOUBLE PRECISION,

    CONSTRAINT fk_workouts_users
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);