-- +goose Up
CREATE TABLE IF NOT EXISTS metrics (
  id     TEXT PRIMARY KEY,
  mtype  TEXT NOT NULL CHECK (mtype IN ('gauge','counter')),
  value  DOUBLE PRECISION,
  delta  BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS metrics_mtype_idx ON metrics(mtype);

-- +goose Down
DROP TABLE IF EXISTS metrics;