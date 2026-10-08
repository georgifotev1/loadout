// Package config reads the process configuration from environment variables.
//
// Everything the binary needs to know about its environment comes from here, so there is one
// place to look for "which knobs exist" and one place that validates them at startup.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
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

	Jobs struct {
		// StopTimeout is how long running jobs get to finish after SIGTERM before they are
		// cancelled (and retried later). It runs alongside the HTTP shutdown, and the hard stop
		// after it takes up to 1s more, so keep it under Cloud Run's 10s.
		StopTimeout time.Duration
	}

	Vali struct {
		// URL is the supplier API that `loadout sync` pulls products from. Until the dealership
		// is signed it's cmd/mock-vali (http://localhost:8081 by default in development).
		URL string
	}

	Auth struct {
		// TOTPKey encrypts admins' TOTP secrets in the database (AES-256, 32 bytes, base64 in
		// TOTP_KEY). Losing it means every admin enrols again (`loadout admin reset-totp`).
		TOTPKey []byte
		// SkipTOTP lets admins use the admin API without the second factor, for local
		// development. Refused in production.
		SkipTOTP bool
	}

	// SiteURL is the storefront's public origin (https://loadout.bg), for links in emails.
	SiteURL string

	Mail struct {
		// SMTPAddr is the mail server's host:port: Mailpit locally, the provider's relay in
		// production.
		SMTPAddr string
		// SMTPUsername and SMTPPassword log in to it; empty for Mailpit.
		SMTPUsername string
		SMTPPassword string
		// From is the sender of every email, e.g. "Loadout <noreply@loadout.bg>".
		From string
	}

	Stripe struct {
		// SecretKey calls Stripe's API: best a restricted key (rk_…) that can only write
		// Checkout Sessions and Refunds. Empty (development only) turns card payments off.
		SecretKey string
		// WebhookSecret verifies the webhook's signatures (whsec_…): the endpoint's in Stripe's
		// Dashboard, or the one `stripe listen` prints locally.
		WebhookSecret string
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

	cfg.Jobs.StopTimeout = duration(getenv, "JOBS_STOP_TIMEOUT", 7*time.Second, &errs)

	// No default in production: syncing from localhost there would be a silent mistake.
	defaultValiURL := ""
	if cfg.Env == EnvDevelopment {
		defaultValiURL = "http://localhost:8081"
	}
	cfg.Vali.URL = str(getenv, "VALI_URL", defaultValiURL)
	if cfg.Vali.URL == "" {
		errs = append(errs, errors.New("VALI_URL: required in production"))
	} else if u, err := url.Parse(cfg.Vali.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("VALI_URL: must be an http(s) URL, got %q", cfg.Vali.URL))
	}

	// A fixed key in development, so local admins keep working across restarts. Production
	// must set its own: this one is in the repository.
	defaultTOTPKey := ""
	if cfg.Env == EnvDevelopment {
		defaultTOTPKey = devTOTPKey
	}
	if v := str(getenv, "TOTP_KEY", defaultTOTPKey); v == "" {
		errs = append(errs, errors.New("TOTP_KEY: required in production"))
	} else if key, err := base64.StdEncoding.DecodeString(v); err != nil || len(key) != 32 {
		errs = append(errs, errors.New("TOTP_KEY: must be 32 bytes in base64 (openssl rand -base64 32)"))
	} else if cfg.Env == EnvProduction && v == devTOTPKey {
		errs = append(errs, errors.New("TOTP_KEY: the development key can't be used in production"))
	} else {
		cfg.Auth.TOTPKey = key
	}
	cfg.Auth.SkipTOTP = boolean(getenv, "AUTH_SKIP_TOTP", &errs)
	if cfg.Auth.SkipTOTP && cfg.Env == EnvProduction {
		errs = append(errs, errors.New("AUTH_SKIP_TOTP: not allowed in production"))
	}

	// Development defaults point at the web dev server and Mailpit (make up). Production must
	// say where it is and how it sends email.
	defaultSiteURL, defaultSMTPAddr, defaultFrom := "", "", ""
	if cfg.Env == EnvDevelopment {
		defaultSiteURL = "http://localhost:3000"
		defaultSMTPAddr = "localhost:1025"
		defaultFrom = "Loadout <noreply@loadout.localhost>"
	}
	cfg.SiteURL = strings.TrimSuffix(str(getenv, "SITE_URL", defaultSiteURL), "/")
	if cfg.SiteURL == "" {
		errs = append(errs, errors.New("SITE_URL: required in production"))
	} else if u, err := url.Parse(cfg.SiteURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" {
		errs = append(errs, fmt.Errorf("SITE_URL: must be an http(s) origin, got %q", cfg.SiteURL))
	}
	cfg.Mail.SMTPAddr = str(getenv, "SMTP_ADDR", defaultSMTPAddr)
	if cfg.Mail.SMTPAddr == "" {
		errs = append(errs, errors.New("SMTP_ADDR: required in production"))
	} else if _, _, err := net.SplitHostPort(cfg.Mail.SMTPAddr); err != nil {
		errs = append(errs, fmt.Errorf("SMTP_ADDR: must be host:port, got %q", cfg.Mail.SMTPAddr))
	}
	cfg.Mail.SMTPUsername = str(getenv, "SMTP_USERNAME", "")
	cfg.Mail.SMTPPassword = str(getenv, "SMTP_PASSWORD", "")
	cfg.Mail.From = str(getenv, "MAIL_FROM", defaultFrom)
	if cfg.Mail.From == "" {
		errs = append(errs, errors.New("MAIL_FROM: required in production"))
	} else if _, err := mail.ParseAddress(cfg.Mail.From); err != nil {
		errs = append(errs, fmt.Errorf("MAIL_FROM: must be an email address, got %q", cfg.Mail.From))
	}

	// Card payments need both or neither. Development can go without: checkout offers cash on
	// delivery only then.
	cfg.Stripe.SecretKey = str(getenv, "STRIPE_SECRET_KEY", "")
	cfg.Stripe.WebhookSecret = str(getenv, "STRIPE_WEBHOOK_SECRET", "")
	switch {
	case cfg.Stripe.SecretKey == "" && cfg.Stripe.WebhookSecret == "" && cfg.Env == EnvProduction:
		errs = append(errs, errors.New("STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET: required in production"))
	case cfg.Stripe.SecretKey == "" && cfg.Stripe.WebhookSecret == "":
	case !strings.HasPrefix(cfg.Stripe.SecretKey, "rk_") && !strings.HasPrefix(cfg.Stripe.SecretKey, "sk_"):
		errs = append(errs, errors.New("STRIPE_SECRET_KEY: must be a restricted (rk_) or secret (sk_) key"))
	case !strings.HasPrefix(cfg.Stripe.WebhookSecret, "whsec_"):
		errs = append(errs, errors.New("STRIPE_WEBHOOK_SECRET: must be a webhook signing secret (whsec_)"))
	}

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

// devTOTPKey is TOTP_KEY's development default.
const devTOTPKey = "bG9hZG91dC1kZXYtdG90cC1rZXktbm90LXNlY3JldCE=" // "loadout-dev-totp-key-not-secret!"

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

func boolean(getenv func(string) string, key string, errs *[]error) bool {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: must be true or false, got %q", key, v))
	}
	return b
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
