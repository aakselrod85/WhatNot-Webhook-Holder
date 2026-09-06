CREATE TABLE layout_image (
    id         BIGSERIAL   PRIMARY KEY,
    channel_id BIGINT      NOT NULL,
    name       TEXT        NOT NULL,
    url        TEXT        NOT NULL,
    width      INT         NOT NULL DEFAULT 0,
    height     INT         NOT NULL DEFAULT 0,
    size_bytes BIGINT      NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX layout_image_channel_id_idx ON layout_image (channel_id, created_at DESC);
