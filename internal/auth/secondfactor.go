package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gfotev/loadout/internal/platform/db"
)

// Admins sign in with a password and then a TOTP code (ADR 0013). After the password their
// session is in SecondFactorEnroll or SecondFactorVerify, and only reaches SecondFactorVerified,
// which the admin API requires, through ConfirmTOTP or VerifySecondFactor. Both start a new
// session, the way Login does, since the session gains rights.

// EnrollTOTP makes a new TOTP secret for an admin who has none yet. It isn't active until
// ConfirmTOTP gets a code from it; calling EnrollTOTP again replaces it. An admin who already
// has TOTP gets ErrSecondFactorState: replacing it takes `loadout admin reset-totp`, so a stolen
// password alone can't swap in the thief's phone.
func (s *Service) EnrollTOTP(ctx context.Context, token string) (TOTPSetup, error) {
	sess, err := s.adminSession(ctx, token, SecondFactorEnroll)
	if err != nil {
		return TOTPSetup{}, err
	}
	secret := newTOTPSecret()
	ok, err := s.store.setPendingTOTP(ctx, s.pool, sess.User.ID, s.sealer.seal(secret, sess.User.ID[:]))
	if err != nil {
		return TOTPSetup{}, err
	}
	if !ok {
		return TOTPSetup{}, ErrSecondFactorState
	}
	return TOTPSetup{Secret: b32.EncodeToString(secret), URI: totpURI(secret, sess.User.Email)}, nil
}

// ConfirmTOTP turns on the secret from EnrollTOTP once code shows the app has it. It returns a
// new, verified session and the admin's recovery codes, which are shown only this once.
func (s *Service) ConfirmTOTP(ctx context.Context, token, code string) (Session, []string, error) {
	sess, err := s.adminSession(ctx, token, SecondFactorEnroll)
	if err != nil {
		return Session{}, nil, err
	}
	key := codeLimitKey(sess.User.ID)
	if err := s.checkLock(ctx, key); err != nil {
		return Session{}, nil, err
	}
	sealed, confirmed, _, ok, err := s.store.totp(ctx, s.pool, sess.User.ID)
	if err != nil {
		return Session{}, nil, err
	}
	if !ok || confirmed {
		return Session{}, nil, ErrSecondFactorState
	}
	secret, err := s.sealer.open(sealed, sess.User.ID[:])
	if err != nil {
		return Session{}, nil, err
	}
	now := s.now()
	step, ok := matchTOTP(secret, normalizeTOTPCode(code), now, 0)
	if !ok {
		return Session{}, nil, s.codeFailed(ctx, key)
	}

	codes := newRecoveryCodes()
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ok, err := s.store.confirmTOTP(ctx, tx, sess.User.ID, step, now)
		if err != nil {
			return err
		}
		if !ok {
			return ErrSecondFactorState
		}
		if err := s.store.replaceRecoveryCodes(ctx, tx, sess.User.ID, hashRecoveryCodes(codes)); err != nil {
			return err
		}
		if err := s.store.clearFailures(ctx, tx, key); err != nil {
			return err
		}
		sess, err = s.rotate(ctx, tx, token, sess, now)
		return err
	})
	if err != nil {
		return Session{}, nil, err
	}
	return sess, codes, nil
}

// VerifySecondFactor takes a TOTP code or a recovery code and returns a new, verified session.
// Each code works once. Wrong codes are limited like failed logins, per admin.
func (s *Service) VerifySecondFactor(ctx context.Context, token, code string) (Session, error) {
	sess, err := s.adminSession(ctx, token, SecondFactorVerify)
	if err != nil {
		return Session{}, err
	}
	key := codeLimitKey(sess.User.ID)
	if err := s.checkLock(ctx, key); err != nil {
		return Session{}, err
	}
	now := s.now()

	// use marks the code used inside the transaction, so two requests can't both spend it.
	var use func(tx pgx.Tx) (bool, error)
	if totpCode := normalizeTOTPCode(code); len(totpCode) == totpDigits {
		sealed, _, lastStep, _, err := s.store.totp(ctx, s.pool, sess.User.ID)
		if err != nil {
			return Session{}, err
		}
		secret, err := s.sealer.open(sealed, sess.User.ID[:])
		if err != nil {
			return Session{}, err
		}
		step, ok := matchTOTP(secret, totpCode, now, lastStep)
		if !ok {
			return Session{}, s.codeFailed(ctx, key)
		}
		use = func(tx pgx.Tx) (bool, error) {
			return s.store.useTOTPStep(ctx, tx, sess.User.ID, step)
		}
	} else {
		rc := normalizeRecoveryCode(code)
		if rc == "" {
			return Session{}, s.codeFailed(ctx, key)
		}
		use = func(tx pgx.Tx) (bool, error) {
			return s.store.useRecoveryCode(ctx, tx, sess.User.ID, hashRecoveryCode(rc), now)
		}
	}

	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ok, err := use(tx)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidCode
		}
		if err := s.store.clearFailures(ctx, tx, key); err != nil {
			return err
		}
		sess, err = s.rotate(ctx, tx, token, sess, now)
		return err
	})
	if errors.Is(err, ErrInvalidCode) {
		return Session{}, s.codeFailed(ctx, key)
	}
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// RegenerateRecoveryCodes replaces a verified admin's recovery codes, used or not, with new ones.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, token string) ([]string, error) {
	sess, err := s.adminSession(ctx, token, SecondFactorVerified)
	if err != nil {
		return nil, err
	}
	codes := newRecoveryCodes()
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.store.replaceRecoveryCodes(ctx, tx, sess.User.ID, hashRecoveryCodes(codes))
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// ResetTOTP removes a user's TOTP and recovery codes and logs them out everywhere, for an admin
// who lost their phone and their codes. Their next login enrols again. It's only run from the
// CLI.
func (s *Service) ResetTOTP(ctx context.Context, email string) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := s.store.userIDByEmail(ctx, tx, strings.TrimSpace(email))
		if err != nil {
			return err
		}
		if err := s.store.deleteTOTP(ctx, tx, id); err != nil {
			return err
		}
		if err := s.store.replaceRecoveryCodes(ctx, tx, id, nil); err != nil {
			return err
		}
		return s.store.deleteUserSessions(ctx, tx, id)
	})
}

// adminSession returns token's session if it's an admin's in the state want.
func (s *Service) adminSession(ctx context.Context, token string, want SecondFactor) (Session, error) {
	sess, err := s.Session(ctx, token)
	if err != nil {
		return Session{}, err
	}
	if sess.User.Role != RoleAdmin {
		return Session{}, ErrAdminsOnly
	}
	if sess.SecondFactor != want {
		return Session{}, ErrSecondFactorState
	}
	return sess, nil
}

// rotate replaces the session for oldToken with a new, verified one that ends when it would have.
func (s *Service) rotate(ctx context.Context, tx pgx.Tx, oldToken string, sess Session, now time.Time) (Session, error) {
	if err := s.store.deleteSession(ctx, tx, hashToken(oldToken)); err != nil {
		return Session{}, err
	}
	token, idHash := newToken()
	sess.Token = token
	sess.secondFactorAt = &now
	sess.SecondFactor = SecondFactorVerified
	if err := s.store.insertSession(ctx, tx, idHash, sess, now); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// codeLimitKey is the login_attempts key for an admin's wrong codes. It gets the email limit:
// 5 free tries, then growing locks. With 3 valid codes in a million at a time, that's no way in.
func codeLimitKey(userID uuid.UUID) string {
	return "code:" + userID.String()
}

func (s *Service) checkLock(ctx context.Context, key string) error {
	now := s.now()
	until, err := s.store.lockedUntil(ctx, s.pool, []string{key}, now)
	if err != nil {
		return err
	}
	if until.After(now) {
		return &LockedError{RetryAfter: until.Sub(now)}
	}
	return nil
}

// codeFailed records a wrong code and returns ErrInvalidCode.
func (s *Service) codeFailed(ctx context.Context, key string) error {
	if err := s.recordFailure(ctx, []string{key}); err != nil {
		return err
	}
	return ErrInvalidCode
}

// normalizeTOTPCode drops the spaces apps show in the middle of a code ("123 456").
func normalizeTOTPCode(code string) string {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	for _, c := range code {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return code
}

func hashRecoveryCodes(codes []string) [][]byte {
	hashes := make([][]byte, len(codes))
	for i, c := range codes {
		hashes[i] = hashRecoveryCode(normalizeRecoveryCode(c))
	}
	return hashes
}
