package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/validate"
)

// tokenPurpose is what an emailed link's token is for. It's also auth.email_tokens.purpose.
type tokenPurpose string

const (
	purposeVerifyEmail   tokenPurpose = "verify_email"
	purposeResetPassword tokenPurpose = "reset_password"
)

// tokenTTL is how long an emailed link works. A reset link is short-lived because it's as good
// as the password.
func tokenTTL(p tokenPurpose) time.Duration {
	if p == purposeResetPassword {
		return time.Hour
	}
	return 48 * time.Hour
}

// Register signs a customer up. It answers the same whether or not the email is taken, so it
// can't be used to find out who has an account: a new account gets an email with a link to
// verify the address, and a taken address gets one saying it already has an account. The new
// account works at once, verified or not; it isn't logged in, as a taken email isn't either.
func (s *Service) Register(ctx context.Context, email, password string) error {
	email = strings.TrimSpace(email)
	var v validate.Validator
	checkEmail(&v, "email", email)
	checkPassword(&v, "password", password)
	if err := v.Err(); err != nil {
		return err
	}

	// Hashed either way, so a taken email takes as long as a new one.
	hash, err := s.hash(ctx, password)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		u, ok, err := s.store.insertUser(ctx, tx, email, hash, RoleCustomer, nil)
		if err != nil {
			return err
		}
		if ok {
			return s.enqueueEmail(ctx, tx, u.ID, EmailVerify)
		}
		id, err := s.store.userIDByEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		return s.enqueueEmail(ctx, tx, id, EmailAccountExists)
	})
}

// ResendVerification emails the session's user a new link to verify their email. It does
// nothing if the email is verified already.
func (s *Service) ResendVerification(ctx context.Context, token string) error {
	sess, err := s.Session(ctx, token)
	if err != nil {
		return err
	}
	if sess.User.EmailVerified {
		return nil
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.enqueueEmail(ctx, tx, sess.User.ID, EmailVerify)
	})
}

// VerifyEmail marks the email of the link's user verified. A wrong, expired or used token is a
// validation error on "token". It doesn't log in: whoever opens the link proves they read the
// mailbox, not that they know the password.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	now := s.now()
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, ok, err := s.store.emailToken(ctx, tx, hashToken(token), purposeVerifyEmail, now, true)
		if err != nil {
			return err
		}
		if !ok {
			return invalidToken()
		}
		return s.store.markEmailVerified(ctx, tx, id, now)
	})
}

// RequestPasswordReset emails a link to set a new password, if the email has an account. It
// answers the same either way.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	var v validate.Validator
	checkEmail(&v, "email", email)
	if err := v.Err(); err != nil {
		return err
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := s.store.userIDByEmail(ctx, tx, email)
		if errors.Is(err, ErrNoSuchUser) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.enqueueEmail(ctx, tx, id, EmailResetPassword)
	})
}

// ResetPasswordInput sets a new password from a reset link. OldToken is the browser's session,
// if any, which ends with the others.
type ResetPasswordInput struct {
	Token    string
	Password string
	OldToken string
}

// ResetPassword sets a new password, logs the user out everywhere and starts a new session. The
// link also verifies the email, and clears the failed logins that may have locked it. A wrong,
// expired or used token is a validation error on "token".
func (s *Service) ResetPassword(ctx context.Context, in ResetPasswordInput) (Session, error) {
	var v validate.Validator
	checkPassword(&v, "password", in.Password)
	if err := v.Err(); err != nil {
		return Session{}, err
	}
	now := s.now()
	tokenHash := hashToken(in.Token)
	// Check the token before the slow hash; it's used up in the transaction below.
	if _, ok, err := s.store.emailToken(ctx, s.pool, tokenHash, purposeResetPassword, now, false); err != nil {
		return Session{}, err
	} else if !ok {
		return Session{}, invalidToken()
	}
	hash, err := s.hash(ctx, in.Password)
	if err != nil {
		return Session{}, err
	}

	var sess Session
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, ok, err := s.store.emailToken(ctx, tx, tokenHash, purposeResetPassword, now, true)
		if err != nil {
			return err
		}
		if !ok {
			return invalidToken()
		}
		u, _, enrolled, err := s.store.userByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.store.markEmailVerified(ctx, tx, id, now); err != nil {
			return err
		}
		u.EmailVerified = true
		if err := s.store.setPasswordHash(ctx, tx, id, hash); err != nil {
			return err
		}
		if err := s.store.clearFailures(ctx, tx, emailLimitKey(u.Email)); err != nil {
			return err
		}
		if err := s.store.deleteUserSessions(ctx, tx, id); err != nil {
			return err
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

// ChangePasswordInput changes the logged-in user's password.
type ChangePasswordInput struct {
	Token           string // the session
	CurrentPassword string
	NewPassword     string
}

// ChangePassword sets a new password after checking the current one, logs the user out of
// their other sessions and replaces this one. A wrong current password is a validation error on
// "current_password" and counts as a failed login for the email.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) (Session, error) {
	sess, err := s.Session(ctx, in.Token)
	if err != nil {
		return Session{}, err
	}
	var v validate.Validator
	v.Check(in.CurrentPassword != "", "current_password", "required")
	v.Check(len(in.CurrentPassword) <= 4*MaxPasswordLen, "current_password", "too_long")
	checkPassword(&v, "new_password", in.NewPassword)
	if err := v.Err(); err != nil {
		return Session{}, err
	}

	key := emailLimitKey(sess.User.Email)
	if err := s.checkLock(ctx, key); err != nil {
		return Session{}, err
	}
	_, current, enrolled, err := s.store.userByID(ctx, s.pool, sess.User.ID)
	if err != nil {
		return Session{}, err
	}
	ok, _, err := s.verify(ctx, in.CurrentPassword, current)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		if err := s.recordFailure(ctx, []string{key}); err != nil {
			return Session{}, err
		}
		v.Add("current_password", "incorrect")
		return Session{}, v.Err()
	}
	hash, err := s.hash(ctx, in.NewPassword)
	if err != nil {
		return Session{}, err
	}

	now := s.now()
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.setPasswordHash(ctx, tx, sess.User.ID, hash); err != nil {
			return err
		}
		if err := s.store.deleteUserSessions(ctx, tx, sess.User.ID); err != nil {
			return err
		}
		// An admin who passed the second factor stays past it.
		sess, err = s.startSession(ctx, tx, sess.User, enrolled, sess.secondFactorAt, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// startSession stores a new session for u. secondFactorAt is when it passed the second factor,
// nil if it hasn't.
func (s *Service) startSession(ctx context.Context, tx pgx.Tx, u User, enrolled bool, secondFactorAt *time.Time, now time.Time) (Session, error) {
	token, idHash := newToken()
	sess := Session{
		Token:          token,
		User:           u,
		ExpiresAt:      now.Add(sessionTTL(u.Role)),
		SecondFactor:   secondFactor(u.Role, enrolled, secondFactorAt),
		secondFactorAt: secondFactorAt,
	}
	if err := s.store.insertSession(ctx, tx, idHash, sess, now); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// issueEmailToken makes a new token for an emailed link, replacing the user's unused ones for
// the same purpose.
func (s *Service) issueEmailToken(ctx context.Context, userID uuid.UUID, p tokenPurpose) (string, error) {
	token, hash := newToken()
	now := s.now()
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.store.replaceEmailToken(ctx, tx, hash, userID, p, now, now.Add(tokenTTL(p)))
	})
	return token, err
}

func (s *Service) enqueueEmail(ctx context.Context, tx pgx.Tx, userID uuid.UUID, e Email) error {
	_, err := s.jobs.InsertTx(ctx, tx, EmailArgs{UserID: userID, Email: e}, nil)
	return err
}

// emailLimitKey is the login_attempts key for an email's failed logins.
func emailLimitKey(email string) string {
	return "email:" + strings.ToLower(email)
}

func invalidToken() error {
	var v validate.Validator
	v.Add("token", "invalid")
	return v.Err()
}
