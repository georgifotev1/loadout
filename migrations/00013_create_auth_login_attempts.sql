-- +goose Up
-- Failed logins per key ("email:<address>" or "ip:<address>"), kept in Postgres so the limit
-- holds across instances. failures counts the failures in a row; they're forgotten an hour after
-- the last one. Past the free tries, each failure locks the key until locked_until, for longer
-- each time.
CREATE TABLE auth.login_attempts (
    key            text PRIMARY KEY,
    failures       integer NOT NULL,
    last_failed_at timestamptz NOT NULL,
    locked_until   timestamptz
);

-- +goose Down
DROP TABLE auth.login_attempts;
