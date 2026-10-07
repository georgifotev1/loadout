-- +goose Up
-- Customers sign up themselves now. An account works before its email is verified; the email
-- only counts as the customer's (e.g. to link guest orders to it) once it is.
ALTER TABLE auth.users ADD COLUMN email_verified_at timestamptz;

-- Single-use tokens for the links in auth emails (verify the email, reset the password). Only
-- the token's SHA-256 is stored, like auth.sessions.
CREATE TABLE auth.email_tokens (
    token_hash bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX email_tokens_user_id_idx ON auth.email_tokens (user_id, purpose);

-- +goose Down
DROP TABLE auth.email_tokens;
ALTER TABLE auth.users DROP COLUMN email_verified_at;
