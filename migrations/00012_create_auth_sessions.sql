-- +goose Up
-- Server-side sessions. The cookie holds a random token; only its SHA-256 is stored, so a leaked
-- database row can't be used as a cookie.
CREATE TABLE auth.sessions (
    id_hash    bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON auth.sessions (user_id);

-- +goose Down
DROP TABLE auth.sessions;
