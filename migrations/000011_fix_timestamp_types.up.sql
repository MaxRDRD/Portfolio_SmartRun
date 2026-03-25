ALTER TABLE users
  ALTER COLUMN created_at SET DATA TYPE timestamptz
    USING created_at AT TIME ZONE 'UTC';


ALTER TABLE workouts
  ALTER COLUMN created_at SET DATA TYPE timestamptz
    USING created_at AT TIME ZONE 'UTC';


ALTER TABLE workouts
  ALTER COLUMN date SET DEFAULT CURRENT_DATE;


ALTER TABLE sessions
  ALTER COLUMN created_at SET DATA TYPE timestamptz
    USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at SET DATA TYPE timestamptz
    USING expires_at AT TIME ZONE 'UTC';


ALTER TABLE password_resets
  ALTER COLUMN expires_at SET DATA TYPE timestamptz
    USING expires_at AT TIME ZONE 'UTC';

