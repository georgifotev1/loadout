// Package auth is email + password login for customers and admins (ADR 0005): users, password
// hashing, server-side sessions, login limits, and the emailed links that verify an email or
// reset a password.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidCredentials is the one answer for a wrong password and an unknown email, so a
	// login can't be used to find out who has an account.
	ErrInvalidCredentials = errors.New("auth: wrong email or password")
	// ErrUnauthenticated means there's no valid session: no token, an unknown one, or expired.
	ErrUnauthenticated = errors.New("auth: not logged in")
	// ErrAdminsOnly means the user isn't an admin, and only admins have a second factor.
	ErrAdminsOnly = errors.New("auth: admins only")
	// ErrInvalidCode is a wrong, expired or already used TOTP or recovery code.
	ErrInvalidCode = errors.New("auth: wrong code")
	// ErrSecondFactorState means the step doesn't fit the session's second factor: enrolling
	// when TOTP is already set up, verifying before it is, and so on (see SecondFactor).
	ErrSecondFactorState = errors.New("auth: not possible in this second-factor state")
	// ErrNoSuchUser is an unknown email in an admin command.
	ErrNoSuchUser = errors.New("auth: no user with this email")
)

// LockedError means too many failed logins for this email or IP. Try again after RetryAfter.
type LockedError struct {
	RetryAfter time.Duration
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("auth: too many failed logins, retry after %s", e.RetryAfter)
}

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

type User struct {
	ID    uuid.UUID
	Email string
	Role  Role
	// EmailVerified is whether the user has opened a link we emailed them (or was created from
	// the CLI, as admins are).
	EmailVerified bool
	CreatedAt     time.Time
}

// SecondFactor is where a session stands with the admins' second factor (TOTP).
type SecondFactor string

const (
	// SecondFactorNotRequired is a customer: there's no second factor.
	SecondFactorNotRequired SecondFactor = "not_required"
	// SecondFactorEnroll is an admin who hasn't set up TOTP yet.
	SecondFactorEnroll SecondFactor = "enroll"
	// SecondFactorVerify is an admin with TOTP whose session hasn't entered a code yet.
	SecondFactorVerify SecondFactor = "verify"
	// SecondFactorVerified is an admin session that entered a code: it may use the admin API.
	SecondFactorVerified SecondFactor = "verified"
)

func secondFactor(r Role, enrolled bool, passedAt *time.Time) SecondFactor {
	switch {
	case r != RoleAdmin:
		return SecondFactorNotRequired
	case !enrolled:
		return SecondFactorEnroll
	case passedAt == nil:
		return SecondFactorVerify
	default:
		return SecondFactorVerified
	}
}

// Session is a logged-in user. Token is only set when a session starts (Login, and the second
// factor steps, which replace it): it's what goes in the cookie, and the database keeps only its
// hash.
type Session struct {
	Token        string
	User         User
	ExpiresAt    time.Time
	SecondFactor SecondFactor
	// secondFactorAt is when this session passed the second factor, if it has.
	secondFactorAt *time.Time
}

// TOTPSetup is what an authenticator app needs: the URI for a QR code, and the same secret in
// base32 for typing it in by hand.
type TOTPSetup struct {
	Secret string
	URI    string
}

// sessionTTL is how long a session lasts from login. Admin sessions are short because they can
// change prices and cancel orders.
func sessionTTL(r Role) time.Duration {
	if r == RoleAdmin {
		return 12 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// newToken returns a random session token and the hash to store for it.
func newToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

// hashToken is a plain SHA-256: the token is 256 random bits, so unlike a password it can't be
// guessed and needs no slow hash.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// limit is how many failed logins a key gets for free, and how long each one after that locks
// it: firstLock, doubling up to maxLock.
type limit struct {
	free      int
	firstLock time.Duration
	maxLock   time.Duration
}

var (
	// An email gets few tries: someone guessing one account's password.
	emailLimit = limit{free: 5, firstLock: 30 * time.Second, maxLock: 15 * time.Minute}
	// An IP gets more: an office or a mobile carrier puts many shoppers behind one address.
	ipLimit = limit{free: 20, firstLock: 30 * time.Second, maxLock: 15 * time.Minute}
)

// failureMemory is how long failed logins are remembered after the last one.
const failureMemory = time.Hour

// lockFor is how long the failures-th failure in a row locks a key.
func (l limit) lockFor(failures int) time.Duration {
	if failures <= l.free {
		return 0
	}
	d := l.firstLock
	for range failures - l.free - 1 {
		d *= 2
		if d >= l.maxLock {
			return l.maxLock
		}
	}
	return d
}
