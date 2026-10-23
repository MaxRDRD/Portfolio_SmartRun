CREATE TABLE IF NOT EXISTS user_passkeys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    attestation_type VARCHAR(64) NOT NULL DEFAULT 'none',
    sign_count BIGINT NOT NULL,
    aaguid BYTEA,
    transports TEXT[] NOT NULL,
    backup_state BOOLEAN DEFAULT FALSE,
    device_name VARCHAR(255) NOT NULL DEFAULT 'Passkey Device',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_passkeys_user_id ON user_passkeys(user_id);
CREATE INDEX IF NOT EXISTS idx_user_passkeys_cred_id ON user_passkeys(credential_id);
