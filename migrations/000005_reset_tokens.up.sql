CREATE TABLE password_resets (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT        NOT NULL REFERENCES users ON DELETE CASCADE,
    token_hash  VARCHAR(128)  NOT NULL,
    expires_at  TIMESTAMPTZ   NOT NULL,
    used        BOOLEAN DEFAULT FALSE,
    UNIQUE (user_id, token_hash)
);