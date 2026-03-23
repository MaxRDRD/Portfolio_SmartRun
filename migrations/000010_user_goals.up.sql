CREATE TABLE user_goals (
    id            bigserial PRIMARY KEY,
    user_id       bigint NOT NULL,
    type          text,           -- "distance_5km", "weight_loss_5kg", "run_100km_month"
    target_value  float,
    current_value float DEFAULT 0,
    deadline      date,
    is_active     bool DEFAULT true
);