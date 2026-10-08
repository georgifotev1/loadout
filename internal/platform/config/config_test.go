package config

import (
	"strings"
	"testing"
	"time"
)

// prodKey is a TOTP_KEY for production tests: 32 bytes, not the development key.
const prodKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ShutdownTimeout != 8*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 8s", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("Log.Format = %q, want text", cfg.Log.Format)
	}
	if cfg.DB.StatementTimeout != 5*time.Second {
		t.Errorf("DB.StatementTimeout = %s, want 5s", cfg.DB.StatementTimeout)
	}
	if cfg.Jobs.StopTimeout != 7*time.Second {
		t.Errorf("Jobs.StopTimeout = %s, want 7s", cfg.Jobs.StopTimeout)
	}
	if cfg.Vali.URL != "http://localhost:8081" {
		t.Errorf("Vali.URL = %q, want the local mock-vali", cfg.Vali.URL)
	}
	if len(cfg.Auth.TOTPKey) != 32 || cfg.Auth.SkipTOTP {
		t.Errorf("Auth = %+v, want the development key and the second factor on", cfg.Auth)
	}
	if cfg.SiteURL != "http://localhost:3000" || cfg.Mail.SMTPAddr != "localhost:1025" {
		t.Errorf("SiteURL = %q, SMTPAddr = %q, want the web dev server and Mailpit", cfg.SiteURL, cfg.Mail.SMTPAddr)
	}
}

func TestLoadAuth(t *testing.T) {
	prod := func(m map[string]string) map[string]string {
		m["ENV"] = "production"
		m["VALI_URL"] = "https://vali.example"
		return withRequired(m)
	}
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"key required in production", prod(map[string]string{}), "TOTP_KEY: required"},
		{"dev key refused in production", prod(map[string]string{"TOTP_KEY": devTOTPKey}), "TOTP_KEY: the development key"},
		{"key set in production", prod(map[string]string{"TOTP_KEY": prodKey}), ""},
		{"not base64", map[string]string{"TOTP_KEY": "not base64!"}, "TOTP_KEY: must be 32 bytes"},
		{"too short", map[string]string{"TOTP_KEY": "c2hvcnQ="}, "TOTP_KEY: must be 32 bytes"},
		{"skip in development", map[string]string{"AUTH_SKIP_TOTP": "true"}, ""},
		{"skip refused in production", prod(map[string]string{"TOTP_KEY": prodKey, "AUTH_SKIP_TOTP": "true"}), "AUTH_SKIP_TOTP: not allowed in production"},
		{"skip not a bool", map[string]string{"AUTH_SKIP_TOTP": "yes"}, "AUTH_SKIP_TOTP: must be true or false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["DATABASE_URL"] = "postgres://x"
			cfg, err := load(env(tt.env))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("err = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			case err == nil && cfg.Auth.SkipTOTP != (tt.env["AUTH_SKIP_TOTP"] == "true"):
				t.Errorf("SkipTOTP = %v", cfg.Auth.SkipTOTP)
			}
		})
	}
}

func TestLoadValiURL(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"required in production", map[string]string{"ENV": "production"}, "VALI_URL: required"},
		{"not a URL", map[string]string{"VALI_URL": "localhost:8081"}, "VALI_URL: must be an http(s) URL"},
		{"no host", map[string]string{"VALI_URL": "http://"}, "VALI_URL: must be an http(s) URL"},
		{"set in production", withRequired(map[string]string{"ENV": "production", "VALI_URL": "https://vali.example", "TOTP_KEY": prodKey}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["DATABASE_URL"] = "postgres://x"
			_, err := load(env(tt.env))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("err = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// withMail adds the site and mail settings production requires.
func withRequired(m map[string]string) map[string]string {
	m["SITE_URL"] = "https://loadout.bg"
	m["SMTP_ADDR"] = "smtp.example:587"
	m["MAIL_FROM"] = "Loadout <noreply@loadout.bg>"
	m["STRIPE_SECRET_KEY"] = "rk_live_x"
	m["STRIPE_WEBHOOK_SECRET"] = "whsec_x"
	return m
}

func TestLoadStripe(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"off in development", map[string]string{}, ""},
		{"required in production", map[string]string{"ENV": "production"}, "STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET: required"},
		{"a key without the webhook secret", map[string]string{"STRIPE_SECRET_KEY": "rk_test_x"}, "STRIPE_WEBHOOK_SECRET: must be"},
		{"a webhook secret without the key", map[string]string{"STRIPE_WEBHOOK_SECRET": "whsec_x"}, "STRIPE_SECRET_KEY: must be"},
		{"a publishable key", map[string]string{"STRIPE_SECRET_KEY": "pk_test_x", "STRIPE_WEBHOOK_SECRET": "whsec_x"}, "STRIPE_SECRET_KEY: must be"},
		{"both", map[string]string{"STRIPE_SECRET_KEY": "sk_test_x", "STRIPE_WEBHOOK_SECRET": "whsec_x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["DATABASE_URL"] = "postgres://x"
			_, err := load(env(tt.env))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("err = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMail(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"site required in production", map[string]string{"ENV": "production"}, "SITE_URL: required"},
		{"smtp required in production", map[string]string{"ENV": "production"}, "SMTP_ADDR: required"},
		{"from required in production", map[string]string{"ENV": "production"}, "MAIL_FROM: required"},
		{"site with a path", map[string]string{"SITE_URL": "https://loadout.bg/shop"}, "SITE_URL: must be an http(s) origin"},
		{"site trailing slash", map[string]string{"SITE_URL": "https://loadout.bg/"}, ""},
		{"smtp without port", map[string]string{"SMTP_ADDR": "smtp.example"}, "SMTP_ADDR: must be host:port"},
		{"from not an address", map[string]string{"MAIL_FROM": "Loadout"}, "MAIL_FROM: must be an email address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["DATABASE_URL"] = "postgres://x"
			cfg, err := load(env(tt.env))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("err = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			case err == nil && strings.HasSuffix(cfg.SiteURL, "/"):
				t.Errorf("SiteURL = %q, want no trailing slash", cfg.SiteURL)
			}
		})
	}
}

func TestLoadProductionDefaultsToJSONLogs(t *testing.T) {
	cfg, err := load(env(withRequired(map[string]string{"DATABASE_URL": "postgres://x", "ENV": "production", "VALI_URL": "https://vali.example", "TOTP_KEY": prodKey})))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want json", cfg.Log.Format)
	}
}

func TestLoadAddr(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"cloud run PORT", map[string]string{"PORT": "9000"}, ":9000"},
		{"HTTP_ADDR wins", map[string]string{"PORT": "9000", "HTTP_ADDR": "127.0.0.1:7000"}, "127.0.0.1:7000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.env["DATABASE_URL"] = "postgres://x"
			cfg, err := load(env(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.HTTP.Addr != tt.want {
				t.Errorf("Addr = %q, want %q", cfg.HTTP.Addr, tt.want)
			}
		})
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := load(env(map[string]string{
		"ENV":                   "staging",
		"HTTP_SHUTDOWN_TIMEOUT": "soon",
		"LOG_LEVEL":             "loud",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"ENV", "HTTP_SHUTDOWN_TIMEOUT", "LOG_LEVEL", "DATABASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}
