CREATE TABLE daily_metrics (
    id              bigserial PRIMARY KEY,
    user_id         bigint NOT NULL,
    date            date NOT NULL UNIQUE(user_id, date),
    
    -- Основные нагрузки
    ctl             float,      -- Chronic Training Load (форма)
    atl             float,      -- Acute Training Load (усталость)
    tsb             float,      -- Training Stress Balance (готовность)
    
    -- Твои метрики
    fatigue_score   int,        -- 0-100
    readiness_score int,        -- 0-100 (основное число для пользователя)
    body_battery_avg float,     -- если потом придёт из Garmin
    
    -- Пассивные данные (пока null, потом заполним)
    steps           int,
    total_calories  int,
    sleep_score     int,
    stress_avg      int,
    
    -- Рекомендация (текст или enum)
    recommendation  text,
    
    updated_at      timestamptz DEFAULT now()
);