package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gfotev/loadout/internal/platform/db"
)

// TODO(phase-10): a periodic job that deletes expired sessions and forgotten login attempts.
type store struct{}

const uniqueViolation = "23505"

// insertUser stores a user. It reports false, and stores nothing, if the email is taken.
func (store) insertUser(ctx context.Context, q db.DBTX, email, passwordHash string, role Role) (User, bool, error) {
	query := `
		INSERT INTO auth.users (email, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id, email, role, created_at`

	var u User
	err := q.QueryRow(ctx, query, email, passwordHash, role).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.CreatedAt,
	)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		return User{}, false, nil
	case err != nil:
		return User{}, false, fmt.Errorf("auth: insert user: %w", err)
	}
	return u, true, nil
}

// userByEmail returns the user and their password hash, or ErrInvalidCredentials.
func (store) userByEmail(ctx context.Context, q db.DBTX, email string) (User, string, error) {
	query := `SELECT id, email, role, created_at, password_hash FROM auth.users WHERE email = $1`

	var u User
	var hash string
	err := q.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.CreatedAt,
		&hash,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, "", ErrInvalidCredentials
	case err != nil:
		return User{}, "", fmt.Errorf("auth: user by email: %w", err)
	}
	return u, hash, nil
}

func (store) setPasswordHash(ctx context.Context, q db.DBTX, userID uuid.UUID, hash string) error {
	query := `UPDATE auth.users SET password_hash = $2 WHERE id = $1`
	if _, err := q.Exec(ctx, query, userID, hash); err != nil {
		return fmt.Errorf("auth: set password hash: %w", err)
	}
	return nil
}

func (store) insertSession(ctx context.Context, q db.DBTX, idHash []byte, s Session, now time.Time) error {
	query := `
		INSERT INTO auth.sessions (id_hash, user_id, created_at, expires_at)
		VALUES ($1, $2, $3, $4)`

	if _, err := q.Exec(ctx, query, idHash, s.User.ID, now, s.ExpiresAt); err != nil {
		return fmt.Errorf("auth: insert session: %w", err)
	}
	return nil
}

// session returns the unexpired session with this hash, or ErrUnauthenticated.
func (store) session(ctx context.Context, q db.DBTX, idHash []byte, now time.Time) (Session, error) {
	query := `
		SELECT u.id, u.email, u.role, u.created_at, s.expires_at
		FROM auth.sessions s
		JOIN auth.users u ON u.id = s.user_id
		WHERE s.id_hash = $1 AND s.expires_at > $2`

	var s Session
	err := q.QueryRow(ctx, query, idHash, now).Scan(
		&s.User.ID,
		&s.User.Email,
		&s.User.Role,
		&s.User.CreatedAt,
		&s.ExpiresAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Session{}, ErrUnauthenticated
	case err != nil:
		return Session{}, fmt.Errorf("auth: session: %w", err)
	}
	return s, nil
}

func (store) deleteSession(ctx context.Context, q db.DBTX, idHash []byte) error {
	if _, err := q.Exec(ctx, `DELETE FROM auth.sessions WHERE id_hash = $1`, idHash); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

// lockedUntil returns the latest lock on any of keys that hasn't run out yet, or the zero time.
func (store) lockedUntil(ctx context.Context, q db.DBTX, keys []string, now time.Time) (time.Time, error) {
	query := `
		SELECT max(locked_until)
		FROM auth.login_attempts
		WHERE key = ANY($1) AND locked_until > $2`

	var until *time.Time
	if err := q.QueryRow(ctx, query, keys, now).Scan(&until); err != nil {
		return time.Time{}, fmt.Errorf("auth: login locks: %w", err)
	}
	if until == nil {
		return time.Time{}, nil
	}
	return *until, nil
}

// recordFailure counts a failed login for key and returns its failures in a row. Failures from
// longer than failureMemory before now are forgotten.
func (store) recordFailure(ctx context.Context, q db.DBTX, key string, now time.Time) (int, error) {
	query := `
		INSERT INTO auth.login_attempts AS a (key, failures, last_failed_at)
		VALUES ($1, 1, $2)
		ON CONFLICT (key) DO UPDATE SET
			failures       = CASE WHEN a.last_failed_at > $3 THEN a.failures + 1 ELSE 1 END,
			last_failed_at = $2
		RETURNING failures`

	var failures int
	if err := q.QueryRow(ctx, query, key, now, now.Add(-failureMemory)).Scan(&failures); err != nil {
		return 0, fmt.Errorf("auth: record failed login: %w", err)
	}
	return failures, nil
}

func (store) lock(ctx context.Context, q db.DBTX, key string, until time.Time) error {
	query := `UPDATE auth.login_attempts SET locked_until = $2 WHERE key = $1`
	if _, err := q.Exec(ctx, query, key, until); err != nil {
		return fmt.Errorf("auth: lock login: %w", err)
	}
	return nil
}

func (store) clearFailures(ctx context.Context, q db.DBTX, key string) error {
	if _, err := q.Exec(ctx, `DELETE FROM auth.login_attempts WHERE key = $1`, key); err != nil {
		return fmt.Errorf("auth: clear failed logins: %w", err)
	}
	return nil
}
