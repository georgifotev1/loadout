-- +goose Up
-- One row per supplier sync run, for the admin dashboard and for spotting failing syncs.
CREATE TABLE catalog.sync_runs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    fetched       int NOT NULL DEFAULT 0 CHECK (fetched >= 0),
    changed       int NOT NULL DEFAULT 0 CHECK (changed >= 0),
    errors        int NOT NULL DEFAULT 0 CHECK (errors >= 0),
    status        text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed')),
    error_message text,                             -- why a failed run failed
    -- finished_at is set exactly when the run is no longer running.
    CONSTRAINT sync_runs_finished CHECK ((status = 'running') = (finished_at IS NULL))
);

CREATE INDEX sync_runs_started_at_idx ON catalog.sync_runs (started_at DESC);

-- +goose Down
DROP TABLE catalog.sync_runs;
