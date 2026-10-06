-- +goose Up
-- The auth module owns this schema: users, sessions and login limits.
CREATE SCHEMA auth;

-- Customers and admins share one table and one login (ADR 0005). password_hash is an argon2id
-- string that carries its own parameters, so they can be raised later.
-- TODO(phase-7): name and email_verified_at, with customer sign-up.
CREATE TABLE auth.users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('customer', 'admin')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE auth.users;
DROP SCHEMA auth;
