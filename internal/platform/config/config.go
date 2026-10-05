// Package config reads the process configuration from environment variables.
//
// Everything the binary needs to know about its environment comes from here, so there is one
// place to look for "which knobs exist" and one place that validates them at startup.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
)

type Config struct {
	Env Env

	HTTP struct {
		Addr string
		// ShutdownTimeout bounds how long in-flight requests get to finish after SIGTERM.
		// Cloud Run gives a revision 10s between SIGTERM and SIGKILL, so keep this below that.
		ShutdownTimeout time.Duration
		// DrainDelay is how long /readyz reports "not ready" before the server stops accepting
		// connections, so a load balancer has time to stop routing new requests here.
		DrainDelay time.Duration
	}

	DB struct {
		URL      string
		MaxConns int32
		// StatementTimeout is the most Postgres lets one statement run on an API or worker
		// connection. `loadout migrate` ignores it, since a migration may legitimately take longer.
		StatementTimeout time.Duration
	}

	Log struct {
		Level  string // debug | info | warn | error
		Format string // text | json
	}
}

// Load reads the configuration using os.Getenv.
func Load() (Config, error) {
	return load(os.Getenv)
}

// load takes the lookup function as a parameter so tests don't have to touch the real environment.
func load(getenv func(string) string) (Config, error) {
	var cfg Config
	var errs []error

	cfg.Env = Env(str(getenv, "ENV", string(EnvDevelopment)))
	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		errs = append(errs, fmt.Errorf("ENV: must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.Env))
	}

	// Cloud Run tells the container which port to listen on through PORT. HTTP_ADDR wins if both are set.
	cfg.HTTP.Addr = str(getenv, "HTTP_ADDR", "")
	if cfg.HTTP.Addr == "" {
		cfg.HTTP.Addr = ":" + str(getenv, "PORT", "8080")
	}
	cfg.HTTP.ShutdownTimeout = duration(getenv, "HTTP_SHUTDOWN_TIMEOUT", 8*time.Second, &errs)
	cfg.HTTP.DrainDelay = duration(getenv, "HTTP_DRAIN_DELAY", 0, &errs)

	cfg.DB.URL = str(getenv, "DATABASE_URL", "")
	if cfg.DB.URL == "" {
		errs = append(errs, errors.New("DATABASE_URL: required"))
	}
	cfg.DB.MaxConns = int32(integer(getenv, "DATABASE_MAX_CONNS", 10, &errs))
	cfg.DB.StatementTimeout = duration(getenv, "DATABASE_STATEMENT_TIMEOUT", 5*time.Second, &errs)

	cfg.Log.Level = strings.ToLower(str(getenv, "LOG_LEVEL", "info"))
	switch cfg.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL: unknown level %q", cfg.Log.Level))
	}

	defaultFormat := "text"
	if cfg.Env == EnvProduction {
		defaultFormat = "json"
	}
	cfg.Log.Format = strings.ToLower(str(getenv, "LOG_FORMAT", defaultFormat))
	if cfg.Log.Format != "text" && cfg.Log.Format != "json" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT: must be text or json, got %q", cfg.Log.Format))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func str(getenv func(string) string, key, def string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return def
}

func duration(getenv func(string) string, key string, def time.Duration, errs *[]error) time.Duration {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		*errs = append(*errs, fmt.Errorf("%s: invalid duration %q", key, v))
		return def
	}
	return d
}

func integer(getenv func(string) string, key string, def int, errs *[]error) int {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		*errs = append(*errs, fmt.Errorf("%s: must be a positive integer, got %q", key, v))
		return def
	}
	return n
}
