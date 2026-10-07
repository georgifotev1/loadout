package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/configurator"
	"github.com/gfotev/loadout/internal/notify"
	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/platform/config"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/jobs"
	"github.com/gfotev/loadout/internal/platform/obs"
	"github.com/gfotev/loadout/internal/specs"
	"github.com/gfotev/loadout/internal/vali"
	"github.com/gfotev/loadout/migrations"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

const usage = `usage: loadout <command> [args]

commands:
  serve                     run the HTTP API
  migrate up|down|status    manage the database schema (up also applies River's migrations;
                            down only rolls back our own)
  sync                      pull products from the supplier (Vali, VALI_URL) into the catalog
  specs seed                store the development spec values (fixtures/specs.json) as admin
                            values, so the PC builder has parts; run after sync
  admin create --email <a>  create an admin; asks for the password (or reads it from stdin)
  admin reset-totp --email <a>
                            remove an admin's TOTP and recovery codes and log them out; their
                            next login sets TOTP up again
  version                   print the version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New("invalid usage")

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsage
	}
	cmd, args := args[0], args[1:]

	switch cmd {
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "serve", "migrate", "sync", "specs", "admin":
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("%w: unknown command %q", errUsage, cmd)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := obs.NewLogger(stderr, cfg.Log.Level, cfg.Log.Format).With("version", version)

	opts := db.Options{URL: cfg.DB.URL, MaxConns: cfg.DB.MaxConns, StatementTimeout: cfg.DB.StatementTimeout}
	if cmd == "migrate" {
		opts.StatementTimeout = 0 // building an index on a big table can take minutes
	}
	pool, err := db.Open(ctx, opts)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Services enqueue through an insert-only client; serve also runs a working one (see
	// jobs.Client).
	jobsInserter, err := jobs.New(pool, jobs.Config{Logger: logger.With("component", "jobs")})
	if err != nil {
		return err
	}
	authSvc, err := auth.NewService(pool, cfg.Auth.TOTPKey, jobsInserter)
	if err != nil {
		return err
	}
	specsSvc := specs.NewService(pool)
	// TODO(vali-api): mock-vali until the dealership is signed; the real client replaces it here.
	catalogSvc := catalog.NewService(pool, vali.NewMockClient(cfg.Vali.URL, nil), specsSvc, logger)

	switch cmd {
	case "serve":
		if cfg.Auth.SkipTOTP {
			logger.Warn("AUTH_SKIP_TOTP is on: the admin API doesn't ask for the second factor")
		}
		mailer := notify.NewService(&notify.SMTPSender{
			Addr:     cfg.Mail.SMTPAddr,
			From:     cfg.Mail.From,
			Username: cfg.Mail.SMTPUsername,
			Password: cfg.Mail.SMTPPassword,
		}, cfg.SiteURL)
		ordersSvc := orders.NewService(pool, catalogSvc, jobsInserter)
		workers := river.NewWorkers()
		river.AddWorker(workers, catalog.NewSyncWorker(catalogSvc))
		river.AddWorker(workers, auth.NewEmailWorker(authSvc, mailer))
		river.AddWorker(workers, orders.NewStepWorker(ordersSvc, mailer))
		jobsClient, err := jobs.New(pool, jobs.Config{
			Logger:  logger.With("component", "jobs"),
			Workers: workers,
			Queues: map[string]river.QueueConfig{
				jobs.QueueSync:   {MaxWorkers: 1},
				jobs.QueueEmail:  {MaxWorkers: 2},
				jobs.QueueOrders: {MaxWorkers: 4},
			},
			PeriodicJobs: []*river.PeriodicJob{
				catalog.PeriodicSync(),
			},
			StopTimeout: cfg.Jobs.StopTimeout,
		})
		if err != nil {
			return err
		}
		app := &application{
			cfg:          cfg,
			logger:       logger,
			db:           pool,
			auth:         authSvc,
			catalog:      catalogSvc,
			specs:        specsSvc,
			configurator: configurator.NewService(pool, catalogSvc, specsSvc),
			orders:       ordersSvc,
			jobs:         jobsClient,
		}
		return app.serve(ctx)
	case "migrate":
		return migrate(ctx, pool, args, stdout)
	case "sync":
		if len(args) != 0 {
			return fmt.Errorf("%w: sync takes no arguments", errUsage)
		}
		return syncCatalog(ctx, catalogSvc, stdout)
	case "specs":
		if len(args) != 1 || args[0] != "seed" {
			return fmt.Errorf("%w: specs needs: seed", errUsage)
		}
		return seedSpecs(ctx, fixtures.Specs, catalogSvc, specsSvc, stdout)
	case "admin":
		if len(args) == 0 {
			return fmt.Errorf("%w: admin needs: create or reset-totp --email <address>", errUsage)
		}
		switch args[0] {
		case "create":
			return createAdmin(ctx, authSvc, args[1:], stdin, stdout, stderr)
		case "reset-totp":
			return resetTOTP(ctx, authSvc, args[1:], stdout, stderr)
		}
		return fmt.Errorf("%w: admin needs: create or reset-totp --email <address>", errUsage)
	}
	return nil
}

func syncCatalog(ctx context.Context, svc *catalog.Service, stdout io.Writer) error {
	run, err := svc.Sync(ctx)
	if run.ID != 0 {
		fmt.Fprintf(stdout, "sync run %d %s: fetched %d, changed %d, errors %d\n",
			run.ID, run.Status, run.Fetched, run.Changed, run.Errors)
	}
	return err
}

func migrate(ctx context.Context, pool *pgxpool.Pool, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: migrate needs one of up, down, status", errUsage)
	}
	m, err := db.NewMigrator(pool, migrations.FS)
	if err != nil {
		return err
	}
	defer m.Close()

	switch args[0] {
	case "up":
		if err := m.Up(ctx, stdout); err != nil {
			return err
		}
		return jobs.MigrateUp(ctx, pool, stdout)
	case "down":
		// River's schema isn't ours to roll back step by step; its CLI (`river migrate-down`) can.
		return m.Down(ctx, stdout)
	case "status":
		if err := m.Status(ctx, stdout); err != nil {
			return err
		}
		return jobs.Status(ctx, pool, stdout)
	default:
		return fmt.Errorf("%w: unknown migrate command %q", errUsage, args[0])
	}
}
