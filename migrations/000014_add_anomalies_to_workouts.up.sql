ALTER TABLE workouts ADD COLUMN is_anomalous BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workouts ADD COLUMN anomaly_reason TEXT;