package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/platform/validate"
)

const password = "correct horse battery"

// newTestService returns a service with cheap hashing and a clock the test moves by hand.
func newTestService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	s := NewService(dbtest.New(t))
	s.params = cheap
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, &now
}

func fieldCode(err error, field string) string {
	var verr *validate.Error
	if !errors.As(err, &verr) {
		return ""
	}
	for _, f := range verr.Fields {
		if f.Field == field {
			return f.Code
		}
	}
	return ""
}

func TestCreateAdmin(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()

	u, err := s.CreateAdmin(ctx, " Admin@Loadout.bg ", password)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "Admin@Loadout.bg" || u.Role != RoleAdmin {
		t.Errorf("user %+v", u)
	}

	tests := []struct {
		name, email, pw, field, code string
	}{
		{"taken, in any case", "admin@loadout.bg", password, "email", "taken"},
		{"no email", " ", password, "email", "required"},
		{"not an email", "admin", password, "email", "invalid_email"},
		{"short password", "other@loadout.bg", "short", "password", "too_short"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.CreateAdmin(ctx, tt.email, tt.pw)
			if code := fieldCode(err, tt.field); code != tt.code {
				t.Errorf("err %v, want %s: %s", err, tt.field, tt.code)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	admin, err := s.CreateAdmin(ctx, "admin@loadout.bg", password)
	if err != nil {
		t.Fatal(err)
	}
	login := func(email, pw, ip string) (Session, error) {
		return s.Login(ctx, LoginInput{Email: email, Password: pw, IP: ip})
	}

	t.Run("right password starts a session", func(t *testing.T) {
		sess, err := login("ADMIN@loadout.bg", password, "10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if sess.Token == "" || sess.User.ID != admin.ID || !sess.ExpiresAt.Equal(now.Add(12*time.Hour)) {
			t.Errorf("session %+v", sess)
		}
		got, err := s.Session(ctx, sess.Token)
		if err != nil || got.User.ID != admin.ID {
			t.Errorf("session lookup: %+v, %v", got, err)
		}
	})

	t.Run("wrong password and unknown email look the same", func(t *testing.T) {
		for _, email := range []string{"admin@loadout.bg", "nobody@loadout.bg"} {
			if _, err := login(email, "wrong password", "10.0.0.2"); !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("%s: err %v, want ErrInvalidCredentials", email, err)
			}
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		_, err := login("", "", "10.0.0.1")
		if fieldCode(err, "email") != "required" || fieldCode(err, "password") != "required" {
			t.Errorf("err %v", err)
		}
	})

	t.Run("an expired session is gone", func(t *testing.T) {
		sess, err := login("admin@loadout.bg", password, "10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		*now = now.Add(12 * time.Hour)
		if _, err := s.Session(ctx, sess.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("err %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("login replaces the old session", func(t *testing.T) {
		old, err := login("admin@loadout.bg", password, "10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		sess, err := s.Login(ctx, LoginInput{Email: "admin@loadout.bg", Password: password, OldToken: old.Token})
		if err != nil {
			t.Fatal(err)
		}
		if sess.Token == old.Token {
			t.Fatal("the token didn't change")
		}
		if _, err := s.Session(ctx, old.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("old session: err %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("logout", func(t *testing.T) {
		sess, err := login("admin@loadout.bg", password, "10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		for range 2 { // logging out twice is fine
			if err := s.Logout(ctx, sess.Token); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.Session(ctx, sess.Token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("err %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("old hash parameters are upgraded on login", func(t *testing.T) {
		s.params = params{time: 2, memory: 8 * 1024, threads: 1}
		defer func() { s.params = cheap }()
		if _, err := login("admin@loadout.bg", password, "10.0.0.1"); err != nil {
			t.Fatal(err)
		}
		_, hash, err := s.store.userByEmail(ctx, s.pool, "admin@loadout.bg")
		if err != nil {
			t.Fatal(err)
		}
		if _, p, _ := verifyPassword(password, hash); p != s.params {
			t.Errorf("params %+v, want %+v", p, s.params)
		}
	})
}

func TestLoginLimits(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateAdmin(ctx, "admin@loadout.bg", password); err != nil {
		t.Fatal(err)
	}
	login := func(email, pw, ip string) error {
		_, err := s.Login(ctx, LoginInput{Email: email, Password: pw, IP: ip})
		return err
	}
	fail := func(email, ip string, n int) {
		t.Helper()
		for i := range n {
			if err := login(email, "wrong password", ip); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("failure %d: err %v", i+1, err)
			}
		}
	}
	locked := func(err error) time.Duration {
		var lerr *LockedError
		if !errors.As(err, &lerr) {
			return 0
		}
		return lerr.RetryAfter
	}

	t.Run("an email is locked after 5 failures, even with the right password", func(t *testing.T) {
		fail("admin@loadout.bg", "10.0.0.1", 5)
		if err := login("admin@loadout.bg", "wrong password", "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("6th failure: err %v", err) // still checked; it sets the first lock
		}
		if d := locked(login("Admin@Loadout.bg", password, "10.0.0.9")); d != 30*time.Second {
			t.Fatalf("right password while locked: retry after %s, want 30s", d)
		}

		*now = now.Add(30 * time.Second)
		fail("admin@loadout.bg", "10.0.0.1", 1)
		if d := locked(login("admin@loadout.bg", password, "10.0.0.1")); d != time.Minute {
			t.Fatalf("after the 7th failure: retry after %s, want 1m", d)
		}

		*now = now.Add(time.Minute)
		if err := login("admin@loadout.bg", password, "10.0.0.1"); err != nil {
			t.Fatalf("after the lock: %v", err)
		}
		// A success clears the email's failures: the next mistake is free again.
		fail("admin@loadout.bg", "10.0.0.1", 1)
		if err := login("admin@loadout.bg", password, "10.0.0.1"); err != nil {
			t.Fatalf("after one more failure: %v", err)
		}
	})

	t.Run("unknown emails are limited the same way", func(t *testing.T) {
		fail("nobody@loadout.bg", "10.0.0.2", 6)
		if d := locked(login("nobody@loadout.bg", "wrong password", "10.0.0.2")); d == 0 {
			t.Error("not locked")
		}
	})

	t.Run("failures are forgotten an hour after the last one", func(t *testing.T) {
		fail("forgetful@loadout.bg", "10.0.0.3", 5)
		*now = now.Add(time.Hour)
		fail("forgetful@loadout.bg", "10.0.0.3", 5)
		if err := login("forgetful@loadout.bg", "wrong password", "10.0.0.3"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("err %v, want ErrInvalidCredentials (not locked)", err)
		}
	})

	t.Run("an IP is locked after 20 failures across emails", func(t *testing.T) {
		*now = now.Add(2 * time.Hour)
		for i := range 20 {
			fail("user"+string(rune('a'+i))+"@loadout.bg", "10.0.0.4", 1)
		}
		fail("admin@loadout.bg", "10.0.0.4", 1) // the 21st locks the IP
		if d := locked(login("admin@loadout.bg", password, "10.0.0.4")); d != 30*time.Second {
			t.Errorf("retry after %s, want 30s", d)
		}
		// The account itself isn't locked: another IP can log in, and that doesn't unlock the IP.
		if err := login("admin@loadout.bg", password, "10.0.0.5"); err != nil {
			t.Errorf("other IP: %v", err)
		}
		if d := locked(login("admin@loadout.bg", password, "10.0.0.4")); d == 0 {
			t.Error("the IP was unlocked by a success elsewhere")
		}
	})
}
