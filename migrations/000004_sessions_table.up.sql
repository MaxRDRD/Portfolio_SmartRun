CREATE TABLE sessions (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT        NOT NULL REFERENCES users ON DELETE CASCADE,
    refresh_hash VARCHAR(128)  NOT NULL,
    expires_at  TIMESTAMPTZ   NOT NULL,
    revoked     BOOLEAN       DEFAULT FALSE,
    created_at  TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE (user_id, refresh_hash)
);
