CREATE TABLE overlay_state (
    id         BIGSERIAL   PRIMARY KEY,
    channel_id BIGINT      NOT NULL UNIQUE,
    seq        BIGINT      NOT NULL DEFAULT 0,
    state      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
