// Package auth is email + password login for customers and admins (ADR 0005): users, password
// hashing, server-side sessions and login limits.
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
	ID        uuid.UUID
	Email     string
	Role      Role
	CreatedAt time.Time
}

// Session is a logged-in user. Token is only set by Login: it's what goes in the cookie, and
// the database keeps only its hash.
type Session struct {
	Token     string
	User      User
	ExpiresAt time.Time
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
