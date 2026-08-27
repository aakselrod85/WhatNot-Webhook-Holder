CREATE TABLE layout_config (
    id         BIGSERIAL   PRIMARY KEY,
    channel_id BIGINT      NOT NULL UNIQUE,
    config     JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
