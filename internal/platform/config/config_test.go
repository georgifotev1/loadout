package config

import (
	"strings"
	"testing"
	"time"
)

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
}

func TestLoadProductionDefaultsToJSONLogs(t *testing.T) {
	cfg, err := load(env(map[string]string{"DATABASE_URL": "postgres://x", "ENV": "production"}))
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
