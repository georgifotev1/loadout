package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/gfotev/loadout/internal/platform/jobs"
)

// Email is one of the emails auth sends.
type Email string

const (
	EmailVerify        Email = "verify_email"
	EmailResetPassword Email = "reset_password"
	// EmailAccountExists answers a sign-up with an email that already has an account.
	EmailAccountExists Email = "account_exists"
)

// emailEvery is the most often one user gets the same email: a sign-up or reset form submitted
// again and again sends one email per window, not one per click.
const emailEvery = 5 * time.Minute

// EmailArgs is the job that sends a user one of auth's emails. It holds no token: the worker
// makes the link's token when it sends, so a live token is never in the jobs table.
type EmailArgs struct {
	UserID uuid.UUID `json:"user_id"`
	Email  Email     `json:"email"`
}

func (EmailArgs) Kind() string { return "auth_email" }

func (EmailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       jobs.QueueEmail,
		MaxAttempts: 5,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByPeriod: emailEvery},
	}
}

// Mailer sends auth's emails. Declared here, by the consumer: *notify.Service.
type Mailer interface {
	VerifyEmail(ctx context.Context, to, token string) error
	ResetPassword(ctx context.Context, to, token string) error
	AccountExists(ctx context.Context, to string) error
}

// EmailWorker sends EmailArgs jobs. A retry after the email went out but before the job was
// marked done sends it again, with a new link; only the latest link works.
type EmailWorker struct {
	river.WorkerDefaults[EmailArgs]
	svc    *Service
	mailer Mailer
}

func NewEmailWorker(svc *Service, m Mailer) *EmailWorker {
	return &EmailWorker{svc: svc, mailer: m}
}

func (w *EmailWorker) Work(ctx context.Context, job *river.Job[EmailArgs]) error {
	s := w.svc
	u, _, _, err := s.store.userByID(ctx, s.pool, job.Args.UserID)
	if errors.Is(err, ErrUnauthenticated) {
		return nil // the user is gone
	}
	if err != nil {
		return err
	}

	switch job.Args.Email {
	case EmailVerify:
		if u.EmailVerified {
			return nil
		}
		token, err := s.issueEmailToken(ctx, u.ID, purposeVerifyEmail)
		if err != nil {
			return err
		}
		return w.mailer.VerifyEmail(ctx, u.Email, token)
	case EmailResetPassword:
		token, err := s.issueEmailToken(ctx, u.ID, purposeResetPassword)
		if err != nil {
			return err
		}
		return w.mailer.ResetPassword(ctx, u.Email, token)
	case EmailAccountExists:
		return w.mailer.AccountExists(ctx, u.Email)
	}
	return river.JobCancel(fmt.Errorf("auth: unknown email %q", job.Args.Email))
}
