CREATE TABLE layout_preset (
    id         BIGSERIAL   PRIMARY KEY,
    channel_id BIGINT      NOT NULL,
    name       TEXT        NOT NULL,
    config     JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel_id, name)
);
