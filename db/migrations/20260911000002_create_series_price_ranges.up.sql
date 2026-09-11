CREATE TABLE series_price_ranges (
    id         BIGSERIAL PRIMARY KEY,
    series_id  BIGINT    NOT NULL REFERENCES series(id),
    price_from INTEGER   NOT NULL,
    price_to   INTEGER,                     -- NULL = open-ended ("$500+")
    count      INTEGER   NOT NULL DEFAULT 0
);
CREATE INDEX series_price_ranges_series_id_idx ON series_price_ranges(series_id);
