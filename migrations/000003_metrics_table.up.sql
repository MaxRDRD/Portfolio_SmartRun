CREATE TABLE IF NOT EXISTS metrics(
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    total_workouts INTEGER NOT NULL DEFAULT 0,
    total_distance DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_duration INTEGER NOT NULL DEFAULT 0,
    avg_pace DOUBLE PRECISION NOT NULL DEFAULT 0,
    from_date DATE NOT NULL,
    to_date DATE NOT NULL,
    total_calories BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_metrics_users
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT chk_metrics_date_range
        CHECK (from_date <= to_date)
);

CREATE INDEX IF NOT EXISTS idx_metrics_user_period
    ON metrics(user_id, from_date, to_date);