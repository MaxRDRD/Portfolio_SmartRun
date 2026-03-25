ALTER TABLE daily_metrics
    ADD COLUMN IF NOT EXISTS streak_days int,
    ADD COLUMN IF NOT EXISTS monotony float,
    ADD COLUMN IF NOT EXISTS strain float;
