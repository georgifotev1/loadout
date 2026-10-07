package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

// TODO(phase-10): a periodic job that deletes expired sessions and forgotten login attempts.
type store struct{}

// insertUser stores a user, with their email verified at verifiedAt (nil: not yet). It reports
// false, and stores nothing, if the email is taken. A taken email doesn't fail the statement, so
// it doesn't abort the caller's transaction.
func (store) insertUser(ctx context.Context, q db.DBTX, email, passwordHash string, role Role, verifiedAt *time.Time) (User, bool, error) {
	query := `
		INSERT INTO auth.users (email, password_hash, role, email_verified_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO NOTHING
		RETURNING id, email, role, email_verified_at IS NOT NULL, created_at`

	var u User
	err := q.QueryRow(ctx, query, email, passwordHash, role, verifiedAt).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.EmailVerified,
		&u.CreatedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, false, nil
	case err != nil:
		return User{}, false, fmt.Errorf("auth: insert user: %w", err)
	}
	return u, true, nil
}

// userByEmail returns the user, their password hash and whether they have confirmed TOTP, or
// ErrInvalidCredentials.
func (store) userByEmail(ctx context.Context, q db.DBTX, email string) (User, string, bool, error) {
	query := `
		SELECT u.id, u.email, u.role, u.email_verified_at IS NOT NULL, u.created_at, u.password_hash,
			t.confirmed_at IS NOT NULL
		FROM auth.users u
		LEFT JOIN auth.totp t ON t.user_id = u.id
		WHERE u.email = $1`

	var u User
	var hash string
	var enrolled bool
	err := q.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.EmailVerified,
		&u.CreatedAt,
		&hash,
		&enrolled,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, "", false, ErrInvalidCredentials
	case err != nil:
		return User{}, "", false, fmt.Errorf("auth: user by email: %w", err)
	}
	return u, hash, enrolled, nil
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
		INSERT INTO auth.sessions (id_hash, user_id, created_at, expires_at, second_factor_at)
		VALUES ($1, $2, $3, $4, $5)`

	if _, err := q.Exec(ctx, query, idHash, s.User.ID, now, s.ExpiresAt, s.secondFactorAt); err != nil {
		return fmt.Errorf("auth: insert session: %w", err)
	}
	return nil
}

// session returns the unexpired session with this hash, or ErrUnauthenticated.
func (store) session(ctx context.Context, q db.DBTX, idHash []byte, now time.Time) (Session, error) {
	query := `
		SELECT u.id, u.email, u.role, u.email_verified_at IS NOT NULL, u.created_at, s.expires_at,
			s.second_factor_at, t.confirmed_at IS NOT NULL
		FROM auth.sessions s
		JOIN auth.users u ON u.id = s.user_id
		LEFT JOIN auth.totp t ON t.user_id = u.id
		WHERE s.id_hash = $1 AND s.expires_at > $2`

	var s Session
	var enrolled bool
	err := q.QueryRow(ctx, query, idHash, now).Scan(
		&s.User.ID,
		&s.User.Email,
		&s.User.Role,
		&s.User.EmailVerified,
		&s.User.CreatedAt,
		&s.ExpiresAt,
		&s.secondFactorAt,
		&enrolled,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Session{}, ErrUnauthenticated
	case err != nil:
		return Session{}, fmt.Errorf("auth: session: %w", err)
	}
	s.SecondFactor = secondFactor(s.User.Role, enrolled, s.secondFactorAt)
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

// deleteUserSessions logs the user out everywhere.
func (store) deleteUserSessions(ctx context.Context, q db.DBTX, userID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM auth.sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("auth: delete user sessions: %w", err)
	}
	return nil
}

func (store) userIDByEmail(ctx context.Context, q db.DBTX, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM auth.users WHERE email = $1`, email).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, ErrNoSuchUser
	case err != nil:
		return uuid.Nil, fmt.Errorf("auth: user by email: %w", err)
	}
	return id, nil
}

// setPendingTOTP stores a new, unconfirmed TOTP secret, replacing an unconfirmed one. It reports
// false, and changes nothing, if the user already has confirmed TOTP.
func (store) setPendingTOTP(ctx context.Context, q db.DBTX, userID uuid.UUID, sealed []byte) (bool, error) {
	query := `
		INSERT INTO auth.totp AS t (user_id, secret)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET secret = EXCLUDED.secret, last_step = 0
		WHERE t.confirmed_at IS NULL`

	tag, err := q.Exec(ctx, query, userID, sealed)
	if err != nil {
		return false, fmt.Errorf("auth: set pending totp: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// totp returns the user's sealed secret, whether it's confirmed, and the last used step. ok is
// false if there's none.
func (store) totp(ctx context.Context, q db.DBTX, userID uuid.UUID) (sealed []byte, confirmed bool, lastStep int64, ok bool, err error) {
	query := `SELECT secret, confirmed_at IS NOT NULL, last_step FROM auth.totp WHERE user_id = $1`

	err = q.QueryRow(ctx, query, userID).Scan(
		&sealed,
		&confirmed,
		&lastStep,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, false, 0, false, nil
	case err != nil:
		return nil, false, 0, false, fmt.Errorf("auth: totp: %w", err)
	}
	return sealed, confirmed, lastStep, true, nil
}

// confirmTOTP marks the pending secret confirmed, with step as its first used step. It reports
// false if there's no pending secret any more (another request confirmed it first).
func (store) confirmTOTP(ctx context.Context, q db.DBTX, userID uuid.UUID, step int64, now time.Time) (bool, error) {
	query := `
		UPDATE auth.totp SET confirmed_at = $3, last_step = $2
		WHERE user_id = $1 AND confirmed_at IS NULL`

	tag, err := q.Exec(ctx, query, userID, step, now)
	if err != nil {
		return false, fmt.Errorf("auth: confirm totp: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// useTOTPStep records step as used. It reports false if it, or a later one, already was: the
// code is a replay.
func (store) useTOTPStep(ctx context.Context, q db.DBTX, userID uuid.UUID, step int64) (bool, error) {
	query := `
		UPDATE auth.totp SET last_step = $2
		WHERE user_id = $1 AND confirmed_at IS NOT NULL AND last_step < $2`

	tag, err := q.Exec(ctx, query, userID, step)
	if err != nil {
		return false, fmt.Errorf("auth: use totp step: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (store) deleteTOTP(ctx context.Context, q db.DBTX, userID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM auth.totp WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("auth: delete totp: %w", err)
	}
	return nil
}

// replaceRecoveryCodes deletes the user's recovery codes, used or not, and stores these hashes.
func (store) replaceRecoveryCodes(ctx context.Context, q db.DBTX, userID uuid.UUID, hashes [][]byte) error {
	if _, err := q.Exec(ctx, `DELETE FROM auth.recovery_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("auth: delete recovery codes: %w", err)
	}
	if len(hashes) == 0 {
		return nil
	}
	query := `
		INSERT INTO auth.recovery_codes (code_hash, user_id)
		SELECT h, $2 FROM unnest($1::bytea[]) AS h`

	if _, err := q.Exec(ctx, query, hashes, userID); err != nil {
		return fmt.Errorf("auth: insert recovery codes: %w", err)
	}
	return nil
}

// useRecoveryCode marks the code used. It reports false if the user has no such unused code.
func (store) useRecoveryCode(ctx context.Context, q db.DBTX, userID uuid.UUID, hash []byte, now time.Time) (bool, error) {
	query := `
		UPDATE auth.recovery_codes SET used_at = $3
		WHERE code_hash = $2 AND user_id = $1 AND used_at IS NULL`

	tag, err := q.Exec(ctx, query, userID, hash, now)
	if err != nil {
		return false, fmt.Errorf("auth: use recovery code: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// userByID returns the user, their password hash and whether they have confirmed TOTP, or
// ErrUnauthenticated if there's no such user.
func (store) userByID(ctx context.Context, q db.DBTX, id uuid.UUID) (User, string, bool, error) {
	query := `
		SELECT u.id, u.email, u.role, u.email_verified_at IS NOT NULL, u.created_at, u.password_hash,
			t.confirmed_at IS NOT NULL
		FROM auth.users u
		LEFT JOIN auth.totp t ON t.user_id = u.id
		WHERE u.id = $1`

	var u User
	var hash string
	var enrolled bool
	err := q.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Role,
		&u.EmailVerified,
		&u.CreatedAt,
		&hash,
		&enrolled,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, "", false, ErrUnauthenticated
	case err != nil:
		return User{}, "", false, fmt.Errorf("auth: user by id: %w", err)
	}
	return u, hash, enrolled, nil
}

func (store) markEmailVerified(ctx context.Context, q db.DBTX, userID uuid.UUID, now time.Time) error {
	query := `UPDATE auth.users SET email_verified_at = $2 WHERE id = $1 AND email_verified_at IS NULL`
	if _, err := q.Exec(ctx, query, userID, now); err != nil {
		return fmt.Errorf("auth: mark email verified: %w", err)
	}
	return nil
}

// replaceEmailToken stores a new token for purpose and deletes the user's unused ones for it, so
// only the link in the latest email works.
func (store) replaceEmailToken(ctx context.Context, q db.DBTX, hash []byte, userID uuid.UUID, purpose tokenPurpose, now, expiresAt time.Time) error {
	query := `DELETE FROM auth.email_tokens WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`
	if _, err := q.Exec(ctx, query, userID, purpose); err != nil {
		return fmt.Errorf("auth: delete email tokens: %w", err)
	}
	query = `
		INSERT INTO auth.email_tokens (token_hash, user_id, purpose, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := q.Exec(ctx, query, hash, userID, purpose, now, expiresAt); err != nil {
		return fmt.Errorf("auth: insert email token: %w", err)
	}
	return nil
}

// emailToken returns the user of an unused, unexpired token for purpose. ok is false if there's
// no such token. With use, it also marks the token used; two requests with one token can't both
// get it.
func (store) emailToken(ctx context.Context, q db.DBTX, hash []byte, purpose tokenPurpose, now time.Time, use bool) (uuid.UUID, bool, error) {
	query := `
		SELECT user_id FROM auth.email_tokens
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3`
	if use {
		query = `
			UPDATE auth.email_tokens SET used_at = $3
			WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
			RETURNING user_id`
	}

	var id uuid.UUID
	err := q.QueryRow(ctx, query, hash, purpose, now).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, fmt.Errorf("auth: email token: %w", err)
	}
	return id, true, nil
}
