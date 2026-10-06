package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSecondFactor(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateAdmin(ctx, "admin@loadout.bg", password); err != nil {
		t.Fatal(err)
	}
	login := func() Session {
		t.Helper()
		sess, err := s.Login(ctx, LoginInput{Email: "admin@loadout.bg", Password: password})
		if err != nil {
			t.Fatal(err)
		}
		return sess
	}
	state := func(token string) SecondFactor {
		t.Helper()
		sess, err := s.Session(ctx, token)
		if err != nil {
			t.Fatalf("session: %v", err)
		}
		return sess.SecondFactor
	}

	first := login()
	if first.SecondFactor != SecondFactorEnroll {
		t.Fatalf("after the first login: %s, want enroll", first.SecondFactor)
	}
	if _, err := s.VerifySecondFactor(ctx, first.Token, "123456"); !errors.Is(err, ErrSecondFactorState) {
		t.Errorf("verify before enrolling: err %v", err)
	}

	var secret []byte
	var recovery []string
	var verified Session
	t.Run("enrol", func(t *testing.T) {
		if _, err := s.EnrollTOTP(ctx, first.Token); err != nil {
			t.Fatal(err)
		}
		setup, err := s.EnrollTOTP(ctx, first.Token) // again: replaces the first secret
		if err != nil {
			t.Fatal(err)
		}
		if secret, err = b32.DecodeString(setup.Secret); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(setup.URI, "secret="+setup.Secret) {
			t.Errorf("uri %s", setup.URI)
		}
		var sealed []byte
		if err := s.pool.QueryRow(ctx, `SELECT secret FROM auth.totp`).Scan(&sealed); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(sealed), string(secret)) {
			t.Error("the secret is stored in plain text")
		}

		if _, _, err := s.ConfirmTOTP(ctx, first.Token, "000000"); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("wrong code: err %v", err)
		}
		sess, codes, err := s.ConfirmTOTP(ctx, first.Token, totpCode(secret, totpStep(*now)))
		if err != nil {
			t.Fatal(err)
		}
		if sess.Token == first.Token || sess.SecondFactor != SecondFactorVerified || !sess.ExpiresAt.Equal(first.ExpiresAt) {
			t.Errorf("session %+v", sess)
		}
		if len(codes) != recoveryCodeCount {
			t.Errorf("%d recovery codes", len(codes))
		}
		if _, err := s.Session(ctx, first.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("the session before confirming still works: %v", err)
		}
		if got := state(sess.Token); got != SecondFactorVerified {
			t.Errorf("stored state %s", got)
		}
		verified, recovery = sess, codes
	})

	t.Run("TOTP can't be replaced over the API", func(t *testing.T) {
		if _, err := s.EnrollTOTP(ctx, verified.Token); !errors.Is(err, ErrSecondFactorState) {
			t.Errorf("verified session: err %v", err)
		}
		if _, err := s.EnrollTOTP(ctx, login().Token); !errors.Is(err, ErrSecondFactorState) {
			t.Errorf("password-only session: err %v", err)
		}
	})

	t.Run("a TOTP code works once", func(t *testing.T) {
		sess := login()
		if sess.SecondFactor != SecondFactorVerify {
			t.Fatalf("after login: %s, want verify", sess.SecondFactor)
		}
		// The code confirming enrolment used this step.
		if _, err := s.VerifySecondFactor(ctx, sess.Token, totpCode(secret, totpStep(*now))); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("replayed code: err %v", err)
		}
		*now = now.Add(30 * time.Second)
		code := totpCode(secret, totpStep(*now))
		got, err := s.VerifySecondFactor(ctx, sess.Token, code[:3]+" "+code[3:])
		if err != nil {
			t.Fatal(err)
		}
		if got.SecondFactor != SecondFactorVerified || got.Token == sess.Token || state(got.Token) != SecondFactorVerified {
			t.Errorf("session %+v", got)
		}
		if _, err := s.VerifySecondFactor(ctx, login().Token, code); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("same code in another session: err %v", err)
		}
	})

	t.Run("a recovery code works once", func(t *testing.T) {
		if _, err := s.VerifySecondFactor(ctx, login().Token, strings.ToUpper(recovery[0])); err != nil {
			t.Fatal(err)
		}
		if _, err := s.VerifySecondFactor(ctx, login().Token, recovery[0]); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("used code: err %v", err)
		}
	})

	t.Run("regenerated recovery codes replace the old ones", func(t *testing.T) {
		if _, err := s.RegenerateRecoveryCodes(ctx, login().Token); !errors.Is(err, ErrSecondFactorState) {
			t.Errorf("unverified session: err %v", err)
		}
		codes, err := s.RegenerateRecoveryCodes(ctx, verified.Token)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.VerifySecondFactor(ctx, login().Token, recovery[1]); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("old code: err %v", err)
		}
		if _, err := s.VerifySecondFactor(ctx, login().Token, codes[0]); err != nil {
			t.Errorf("new code: %v", err)
		}
	})

	t.Run("wrong codes lock like failed logins", func(t *testing.T) {
		sess := login()
		for i := range 6 {
			if _, err := s.VerifySecondFactor(ctx, sess.Token, "000000"); !errors.Is(err, ErrInvalidCode) {
				t.Fatalf("failure %d: err %v", i+1, err)
			}
		}
		*now = now.Add(10 * time.Second)
		_, err := s.VerifySecondFactor(ctx, sess.Token, totpCode(secret, totpStep(*now)+1))
		if lerr := (*LockedError)(nil); !errors.As(err, &lerr) || lerr.RetryAfter != 20*time.Second {
			t.Fatalf("right code while locked: err %v", err)
		}
		*now = now.Add(20 * time.Second)
		if _, err := s.VerifySecondFactor(ctx, sess.Token, totpCode(secret, totpStep(*now))); err != nil {
			t.Errorf("after the lock: %v", err)
		}
	})

	t.Run("reset", func(t *testing.T) {
		if err := s.ResetTOTP(ctx, "nobody@loadout.bg"); !errors.Is(err, ErrNoSuchUser) {
			t.Errorf("unknown email: err %v", err)
		}
		if err := s.ResetTOTP(ctx, " Admin@Loadout.bg "); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Session(ctx, verified.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("sessions survived the reset: %v", err)
		}
		if got := login().SecondFactor; got != SecondFactorEnroll {
			t.Errorf("after the reset: %s, want enroll", got)
		}
		var n int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM auth.recovery_codes`).Scan(&n); err != nil || n != 0 {
			t.Errorf("%d recovery codes left, err %v", n, err)
		}
	})
}

func TestSecondFactorIsForAdmins(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateAdmin(ctx, "admin@loadout.bg", password); err != nil {
		t.Fatal(err)
	}
	// Customers can't sign up yet: add one with the admin's password hash.
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO auth.users (email, password_hash, role)
		SELECT 'customer@loadout.bg', password_hash, 'customer' FROM auth.users`); err != nil {
		t.Fatal(err)
	}
	sess, err := s.Login(ctx, LoginInput{Email: "customer@loadout.bg", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	if sess.SecondFactor != SecondFactorNotRequired {
		t.Errorf("customer: %s", sess.SecondFactor)
	}
	if _, err := s.EnrollTOTP(ctx, sess.Token); !errors.Is(err, ErrAdminsOnly) {
		t.Errorf("enrol: err %v", err)
	}
	if _, err := s.VerifySecondFactor(ctx, sess.Token, "123456"); !errors.Is(err, ErrAdminsOnly) {
		t.Errorf("verify: err %v", err)
	}
	if _, err := s.EnrollTOTP(ctx, "no such token"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no session: err %v", err)
	}
}
