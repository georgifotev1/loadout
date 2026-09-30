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

	"github.com/gfotev/loadout/internal/platform/config"
	"github.com/gfotev/loadout/internal/platform/db"
	"github.com/gfotev/loadout/internal/platform/obs"
	"github.com/gfotev/loadout/migrations"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

const usage = `usage: loadout <command> [args]

commands:
  serve                     run the HTTP API
  migrate up|down|status    manage the database schema
  version                   print the version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New("invalid usage")

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsage
	}
	cmd, args := args[0], args[1:]

	switch cmd {
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "serve", "migrate":
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

	pool, err := db.Open(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "serve":
		app := &application{cfg: cfg, logger: logger, db: pool}
		return app.serve(ctx)
	case "migrate":
		return migrate(ctx, pool, args, stdout)
	}
	return nil
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
		return m.Up(ctx, stdout)
	case "down":
		return m.Down(ctx, stdout)
	case "status":
		return m.Status(ctx, stdout)
	default:
		return fmt.Errorf("%w: unknown migrate command %q", errUsage, args[0])
	}
}
