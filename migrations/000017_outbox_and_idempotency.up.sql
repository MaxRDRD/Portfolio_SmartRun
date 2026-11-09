CREATE TABLE IF NOT EXISTS outbox_events(
    id BIGSERIAL PRIMARY KEY,
    event_type    VARCHAR(64) NOT NULL,
    exchange_name VARCHAR(64) NOT NULL DEFAULT '',
    routing_key   VARCHAR(64) NOT NULL,
    payload       JSONB NOT NULL,
    status        VARCHAR(32) NOT NULL DEFAULT 'PENDING'
                  CHECK (status IN ('PENDING', 'PROCESSING', 'PUBLISHED', 'FAILED')),
    retry_count   INT NOT NULL DEFAULT 0,
    max_retries   INT NOT NULL DEFAULT 5,
    last_error    TEXT,
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at  TIMESTAMPTZ
);

-- Индексирует только те записи, которые еще ждут отправки или упали с ошибкой
CREATE INDEX IF NOT EXISTS idx_outbox_events_pending 
    ON outbox_events (scheduled_at, created_at) 
    WHERE status IN ('PENDING', 'FAILED');

CREATE TABLE IF NOT EXISTS processed_events (
    event_id      BIGINT NOT NULL,
    consumer_name VARCHAR(64) NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, consumer_name)
);