package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// maxConcurrentHashes caps how many argon2id hashes run at once. Each takes 64 MiB, so a burst
// of logins would otherwise run the instance out of memory.
const maxConcurrentHashes = 4

type Service struct {
	pool    *pgxpool.Pool
	store   store
	jobs    Inserter
	params  params
	hashing chan struct{}
	sealer  sealer
	now     func() time.Time
	// dummyHash is checked against when the email is unknown, so that answer takes as long as a
	// wrong password and the timing doesn't tell who has an account.
	dummyHash func() string
}

// Inserter enqueues background jobs in the caller's transaction: *jobs.Client.
type Inserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// NewService returns the auth service. totpKey encrypts the admins' TOTP secrets: TOTPKeyLen
// random bytes, kept out of the database. jobs enqueues the emails (EmailArgs).
func NewService(pool *pgxpool.Pool, totpKey []byte, jobs Inserter) (*Service, error) {
	sealer, err := newSealer(totpKey)
	if err != nil {
		return nil, err
	}
	s := &Service{
		pool:    pool,
		jobs:    jobs,
		params:  defaultParams,
		hashing: make(chan struct{}, maxConcurrentHashes),
		sealer:  sealer,
		now:     time.Now,
	}
	s.dummyHash = sync.OnceValue(func() string {
		return hashPassword(rand.Text(), s.params)
	})
	return s, nil
}

// CreateAdmin adds an admin. Admins are only created this way (from the CLI); there's no
// sign-up for them, and their email counts as verified. A taken email is a validation error.
func (s *Service) CreateAdmin(ctx context.Context, email, password string) (User, error) {
	email = strings.TrimSpace(email)
	var v validate.Validator
	checkEmail(&v, "email", email)
	checkPassword(&v, "password", password)
	if err := v.Err(); err != nil {
		return User{}, err
	}

	hash, err := s.hash(ctx, password)
	if err != nil {
		return User{}, err
	}
	now := s.now()
	u, ok, err := s.store.insertUser(ctx, s.pool, email, hash, RoleAdmin, &now)
	if err != nil {
		return User{}, err
	}
	if !ok {
		v.Add("email", "taken")
		return User{}, v.Err()
	}
	return u, nil
}

// LoginInput is a login attempt. IP is the client's address, for the per-IP limit. OldToken is
// the session the browser already had, if any: it's deleted, so each login gets a fresh token.
type LoginInput struct {
	Email    string
	Password string
	IP       string
	OldToken string
}

// Login checks an email and password and starts a session. A wrong password and an unknown email
// both give ErrInvalidCredentials. Too many failures for the email or the IP give a
// *LockedError, even when the password is right, so a lock can't be guessed past.
func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	email := strings.TrimSpace(in.Email)
	var v validate.Validator
	v.Check(email != "", "email", "required")
	v.Check(in.Password != "", "password", "required")
	v.Check(len(in.Password) <= 4*MaxPasswordLen, "password", "too_long")
	if err := v.Err(); err != nil {
		return Session{}, err
	}

	keys := []string{emailLimitKey(email)}
	if in.IP != "" {
		keys = append(keys, "ip:"+in.IP)
	}
	now := s.now()
	until, err := s.store.lockedUntil(ctx, s.pool, keys, now)
	if err != nil {
		return Session{}, err
	}
	if until.After(now) {
		return Session{}, &LockedError{RetryAfter: until.Sub(now)}
	}

	u, hash, enrolled, err := s.store.userByEmail(ctx, s.pool, email)
	known := err == nil
	if errors.Is(err, ErrInvalidCredentials) {
		hash = s.dummyHash()
	} else if err != nil {
		return Session{}, err
	}
	ok, p, err := s.verify(ctx, in.Password, hash)
	if err != nil {
		return Session{}, fmt.Errorf("auth: verify password of user %s: %w", u.ID, err)
	}
	if !known || !ok {
		if err := s.recordFailure(ctx, keys); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidCredentials
	}

	// The parameters were raised since this hash was made: store a new one now that we have the
	// password. Hashing happens before the transaction, which shouldn't wait on it.
	var newHash string
	if p != s.params {
		if newHash, err = s.hash(ctx, in.Password); err != nil {
			return Session{}, err
		}
	}

	var sess Session
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Only the email's failures: logging into one account mustn't reset the limit on an IP
		// that's guessing others.
		if err := s.store.clearFailures(ctx, tx, keys[0]); err != nil {
			return err
		}
		if newHash != "" {
			if err := s.store.setPasswordHash(ctx, tx, u.ID, newHash); err != nil {
				return err
			}
		}
		if in.OldToken != "" {
			if err := s.store.deleteSession(ctx, tx, hashToken(in.OldToken)); err != nil {
				return err
			}
		}
		sess, err = s.startSession(ctx, tx, u, enrolled, nil, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// recordFailure counts a failed login against each key, and locks the ones past their free tries.
func (s *Service) recordFailure(ctx context.Context, keys []string) error {
	now := s.now()
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, key := range keys {
			failures, err := s.store.recordFailure(ctx, tx, key, now)
			if err != nil {
				return err
			}
			l := emailLimit
			if strings.HasPrefix(key, "ip:") {
				l = ipLimit
			}
			if d := l.lockFor(failures); d > 0 {
				if err := s.store.lock(ctx, tx, key, now.Add(d)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Session returns the session for a cookie token, or ErrUnauthenticated.
func (s *Service) Session(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrUnauthenticated
	}
	return s.store.session(ctx, s.pool, hashToken(token), s.now())
}

// Logout ends the session. Logging out of a session that doesn't exist is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.deleteSession(ctx, s.pool, hashToken(token))
}

func (s *Service) hash(ctx context.Context, pw string) (string, error) {
	if err := s.acquire(ctx); err != nil {
		return "", err
	}
	defer s.release()
	return hashPassword(pw, s.params), nil
}

func (s *Service) verify(ctx context.Context, pw, hash string) (bool, params, error) {
	if err := s.acquire(ctx); err != nil {
		return false, params{}, err
	}
	defer s.release()
	return verifyPassword(pw, hash)
}

func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.hashing <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) release() {
	<-s.hashing
}

// checkEmail records a field error unless email (already trimmed) looks like an address. The
// real check is the link we email to it.
func checkEmail(v *validate.Validator, field, email string) {
	v.Check(email != "", field, "required")
	v.Check(len(email) <= 254 && strings.Count(email, "@") == 1 &&
		!strings.HasPrefix(email, "@") && !strings.HasSuffix(email, "@"), field, "invalid_email")
}
