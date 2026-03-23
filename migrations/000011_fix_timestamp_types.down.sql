-- Revert timestamp types back to TIMESTAMP (if needed)

ALTER TABLE users
  ALTER COLUMN created_at SET DATA TYPE timestamp
    USING created_at AT TIME ZONE 'UTC';

ALTER TABLE workouts
  ALTER COLUMN created_at SET DATA TYPE timestamp
    USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN date SET DEFAULT NOW();

ALTER TABLE sessions
  ALTER COLUMN created_at SET DATA TYPE timestamp
    USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at SET DATA TYPE timestamp
    USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE reset_tokens
  ALTER COLUMN created_at SET DATA TYPE timestamp
    USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at SET DATA TYPE timestamp
    USING expires_at AT TIME ZONE 'UTC';
