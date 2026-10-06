-- +goose Up
-- Admins' second factor: a TOTP secret from an authenticator app (ADR 0013). The secret is
-- encrypted with AES-GCM under TOTP_KEY, so a database dump alone can't produce codes.
-- confirmed_at is null while enrolment waits for the first code. last_step is the 30-second
-- step of the last accepted code, so a code can't be used twice.
CREATE TABLE auth.totp (
    user_id      uuid PRIMARY KEY REFERENCES auth.users (id) ON DELETE CASCADE,
    secret       bytea NOT NULL,
    confirmed_at timestamptz,
    last_step    bigint NOT NULL DEFAULT 0
);

-- Single-use codes for when the phone is lost. Only their SHA-256 is stored: they're 80 random
-- bits, so a fast hash is enough.
CREATE TABLE auth.recovery_codes (
    code_hash bytea PRIMARY KEY,
    user_id   uuid NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    used_at   timestamptz
);

CREATE INDEX recovery_codes_user_id_idx ON auth.recovery_codes (user_id);

-- When this session passed the second factor; null until it has.
ALTER TABLE auth.sessions ADD COLUMN second_factor_at timestamptz;

-- +goose Down
ALTER TABLE auth.sessions DROP COLUMN second_factor_at;
DROP TABLE auth.recovery_codes;
DROP TABLE auth.totp;
