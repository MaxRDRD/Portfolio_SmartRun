CREATE TABLE IF NOT EXISTS workouts(
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    distance DECIMAL(7,2) NOT NULL,
    duration INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_workouts_users
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);