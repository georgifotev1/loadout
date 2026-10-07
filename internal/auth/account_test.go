package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// mailbox records what EmailWorker sends.
type mailbox struct {
	sent []sentEmail
}

type sentEmail struct {
	email Email
	to    string
	token string
}

func (m *mailbox) VerifyEmail(_ context.Context, to, token string) error {
	m.sent = append(m.sent, sentEmail{EmailVerify, to, token})
	return nil
}

func (m *mailbox) ResetPassword(_ context.Context, to, token string) error {
	m.sent = append(m.sent, sentEmail{EmailResetPassword, to, token})
	return nil
}

func (m *mailbox) AccountExists(_ context.Context, to string) error {
	m.sent = append(m.sent, sentEmail{EmailAccountExists, to, ""})
	return nil
}

// sendQueued runs EmailWorker on the email jobs waiting in the queue, as River would, and
// deletes them. With the jobs gone, an equal one can be queued again at once, as if the
// emailEvery window had passed.
func sendQueued(t *testing.T, s *Service) []sentEmail {
	t.Helper()
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `DELETE FROM river.river_job WHERE kind = 'auth_email' RETURNING args`)
	if err != nil {
		t.Fatal(err)
	}
	args, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EmailArgs, error) {
		var raw []byte
		var a EmailArgs
		if err := row.Scan(&raw); err != nil {
			return a, err
		}
		return a, json.Unmarshal(raw, &a)
	})
	if err != nil {
		t.Fatal(err)
	}
	m := &mailbox{}
	w := NewEmailWorker(s, m)
	for _, a := range args {
		if err := w.Work(ctx, &river.Job[EmailArgs]{JobRow: &rivertype.JobRow{}, Args: a}); err != nil {
			t.Fatal(err)
		}
	}
	return m.sent
}

func onlyEmail(t *testing.T, sent []sentEmail, want Email, to string) sentEmail {
	t.Helper()
	if len(sent) != 1 || sent[0].email != want || sent[0].to != to {
		t.Fatalf("sent %+v, want one %s to %s", sent, want, to)
	}
	return sent[0]
}

func TestRegister(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	const email = "ivan@example.bg"

	if err := s.Register(ctx, " "+email+" ", password); err != nil {
		t.Fatal(err)
	}
	verify := onlyEmail(t, sendQueued(t, s), EmailVerify, email)

	// The account works before the email is verified.
	sess, err := s.Login(ctx, LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	if sess.User.Role != RoleCustomer || sess.User.EmailVerified || sess.SecondFactor != SecondFactorNotRequired {
		t.Errorf("user %+v, second factor %s", sess.User, sess.SecondFactor)
	}

	t.Run("a taken email gets an email, not an error", func(t *testing.T) {
		if err := s.Register(ctx, "IVAN@example.bg", "another password"); err != nil {
			t.Fatal(err)
		}
		onlyEmail(t, sendQueued(t, s), EmailAccountExists, email)
		if _, err := s.Login(ctx, LoginInput{Email: email, Password: "another password"}); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("the second sign-up's password works: %v", err)
		}
	})

	t.Run("the same email again soon is sent once", func(t *testing.T) {
		for range 3 {
			if err := s.Register(ctx, email, password); err != nil {
				t.Fatal(err)
			}
		}
		onlyEmail(t, sendQueued(t, s), EmailAccountExists, email)
	})

	t.Run("invalid input", func(t *testing.T) {
		err := s.Register(ctx, "not an email", "short")
		if fieldCode(err, "email") != "invalid_email" || fieldCode(err, "password") != "too_short" {
			t.Errorf("err %v", err)
		}
	})

	t.Run("verify the email", func(t *testing.T) {
		*now = now.Add(47 * time.Hour)
		if err := s.VerifyEmail(ctx, verify.token); err != nil {
			t.Fatal(err)
		}
		got, err := s.Session(ctx, sess.Token)
		if err != nil || !got.User.EmailVerified {
			t.Errorf("user %+v, err %v", got.User, err)
		}
		if err := s.VerifyEmail(ctx, verify.token); fieldCode(err, "token") != "invalid" {
			t.Errorf("a used token: err %v", err)
		}
		// Verified: a resend sends nothing.
		if err := s.ResendVerification(ctx, sess.Token); err != nil {
			t.Fatal(err)
		}
		if sent := sendQueued(t, s); len(sent) != 0 {
			t.Errorf("sent %+v", sent)
		}
	})
}

func TestVerifyEmailLinks(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	if err := s.Register(ctx, "ivan@example.bg", password); err != nil {
		t.Fatal(err)
	}
	first := onlyEmail(t, sendQueued(t, s), EmailVerify, "ivan@example.bg")
	sess, err := s.Login(ctx, LoginInput{Email: "ivan@example.bg", Password: password})
	if err != nil {
		t.Fatal(err)
	}

	// A resent link replaces the first one.
	if err := s.ResendVerification(ctx, sess.Token); err != nil {
		t.Fatal(err)
	}
	second := onlyEmail(t, sendQueued(t, s), EmailVerify, "ivan@example.bg")
	if err := s.VerifyEmail(ctx, first.token); fieldCode(err, "token") != "invalid" {
		t.Errorf("the replaced link: err %v", err)
	}

	*now = now.Add(48 * time.Hour)
	if err := s.VerifyEmail(ctx, second.token); fieldCode(err, "token") != "invalid" {
		t.Errorf("an expired link: err %v", err)
	}
	if err := s.VerifyEmail(ctx, "made up"); fieldCode(err, "token") != "invalid" {
		t.Errorf("a made up token: err %v", err)
	}
}

func TestPasswordReset(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	const email = "ivan@example.bg"
	if err := s.Register(ctx, email, password); err != nil {
		t.Fatal(err)
	}
	sendQueued(t, s)
	old, err := s.Login(ctx, LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}

	// An unknown email answers the same, and gets nothing.
	if err := s.RequestPasswordReset(ctx, "nobody@example.bg"); err != nil {
		t.Fatal(err)
	}
	if sent := sendQueued(t, s); len(sent) != 0 {
		t.Errorf("sent %+v", sent)
	}

	if err := s.RequestPasswordReset(ctx, email); err != nil {
		t.Fatal(err)
	}
	link := onlyEmail(t, sendQueued(t, s), EmailResetPassword, email)

	// Failed logins lock the email; the reset unlocks it.
	for range emailLimit.free + 1 {
		_, _ = s.Login(ctx, LoginInput{Email: email, Password: "wrong password"})
	}

	if _, err := s.ResetPassword(ctx, ResetPasswordInput{Token: link.token, Password: "short"}); fieldCode(err, "password") != "too_short" {
		t.Errorf("short password: err %v", err)
	}
	*now = now.Add(59 * time.Minute)
	sess, err := s.ResetPassword(ctx, ResetPasswordInput{Token: link.token, Password: "a new password"})
	if err != nil {
		t.Fatal(err)
	}
	if !sess.User.EmailVerified {
		t.Error("the reset link didn't verify the email")
	}
	if _, err := s.Session(ctx, old.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the old session survived: %v", err)
	}
	if _, err := s.Session(ctx, sess.Token); err != nil {
		t.Errorf("the new session: %v", err)
	}
	if _, err := s.Login(ctx, LoginInput{Email: email, Password: "a new password"}); err != nil {
		t.Errorf("login with the new password: %v", err)
	}
	if _, err := s.ResetPassword(ctx, ResetPasswordInput{Token: link.token, Password: "another password"}); fieldCode(err, "token") != "invalid" {
		t.Errorf("a used link: err %v", err)
	}

	t.Run("a link expires after an hour", func(t *testing.T) {
		if err := s.RequestPasswordReset(ctx, email); err != nil {
			t.Fatal(err)
		}
		link := onlyEmail(t, sendQueued(t, s), EmailResetPassword, email)
		*now = now.Add(time.Hour)
		if _, err := s.ResetPassword(ctx, ResetPasswordInput{Token: link.token, Password: "another password"}); fieldCode(err, "token") != "invalid" {
			t.Errorf("err %v", err)
		}
	})
}

func TestChangePassword(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	if err := s.Register(ctx, "ivan@example.bg", password); err != nil {
		t.Fatal(err)
	}
	other, err := s.Login(ctx, LoginInput{Email: "ivan@example.bg", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.Login(ctx, LoginInput{Email: "ivan@example.bg", Password: password})
	if err != nil {
		t.Fatal(err)
	}

	in := ChangePasswordInput{Token: sess.Token, CurrentPassword: "wrong password", NewPassword: "a new password"}
	if _, err := s.ChangePassword(ctx, in); fieldCode(err, "current_password") != "incorrect" {
		t.Errorf("wrong current password: err %v", err)
	}
	in.CurrentPassword = password
	next, err := s.ChangePassword(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{sess.Token, other.Token} {
		if _, err := s.Session(ctx, old); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("an old session survived: %v", err)
		}
	}
	if _, err := s.Session(ctx, next.Token); err != nil {
		t.Errorf("the new session: %v", err)
	}
	if _, err := s.ChangePassword(ctx, in); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("with the old session: err %v", err)
	}
}

// failingInserter fails every job insert, like a broken jobs table.
type failingInserter struct{}

var errInsert = errors.New("insert failed")

func (failingInserter) InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	return nil, errInsert
}

// The user and their email job commit together: if the job can't be queued, there's no account
// that never got its email either.
func TestRegisterRollsBackWithoutTheEmail(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	s.jobs = failingInserter{}

	if err := s.Register(ctx, "ivan@example.bg", password); !errors.Is(err, errInsert) {
		t.Fatalf("err %v, want the insert error", err)
	}
	if _, err := s.store.userIDByEmail(ctx, s.pool, "ivan@example.bg"); !errors.Is(err, ErrNoSuchUser) {
		t.Errorf("the user was stored: %v", err)
	}
}

func TestEmailWorkerSkipsDeletedUsers(t *testing.T) {
	s, _ := newTestService(t)
	w := NewEmailWorker(s, &mailbox{})
	job := &river.Job[EmailArgs]{JobRow: &rivertype.JobRow{}, Args: EmailArgs{UserID: uuid.New(), Email: EmailVerify}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Errorf("err %v", err)
	}
}
