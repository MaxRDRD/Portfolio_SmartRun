CREATE TABLE password_resets (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT        NOT NULL REFERENCES users ON DELETE CASCADE,
    token_hash VARCHAR(150) NOT NULL,
    expires_at  TIMESTAMPTZ   NOT NULL,
    used        BOOLEAN DEFAULT FALSE,
    UNIQUE (user_id, refresh_hash)
);