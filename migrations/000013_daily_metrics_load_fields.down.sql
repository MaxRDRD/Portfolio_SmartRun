ALTER TABLE daily_metrics
    DROP COLUMN IF EXISTS strain,
    DROP COLUMN IF EXISTS monotony,
    DROP COLUMN IF EXISTS streak_days;
