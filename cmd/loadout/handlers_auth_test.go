package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/platform/config"
	"github.com/gfotev/loadout/internal/platform/db/dbtest"
	"github.com/gfotev/loadout/internal/specs"
)

const testPassword = "correct horse battery"

func newAuthApp(t *testing.T) *application {
	t.Helper()
	app := newTestApp()
	app.db = dbtest.New(t)
	app.auth = auth.NewService(app.db)
	app.specs = specs.NewService(app.db)
	return app
}

// adminCookie creates an admin with a new email and returns a session cookie for them.
func adminCookie(t *testing.T, app *application) *http.Cookie {
	t.Helper()
	ctx := context.Background()
	email := "admin-" + uuid.NewString() + "@loadout.bg"
	if _, err := app.auth.CreateAdmin(ctx, email, testPassword); err != nil {
		t.Fatal(err)
	}
	sess, err := app.auth.Login(ctx, auth.LoginInput{Email: email, Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: app.sessionCookieName(), Value: sess.Token}
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatalf("no session cookie in %v", rec.Header()["Set-Cookie"])
	return nil
}

func TestAuthAPI(t *testing.T) {
	app := newAuthApp(t)
	ctx := context.Background()
	if _, err := app.auth.CreateAdmin(ctx, "admin@loadout.bg", testPassword); err != nil {
		t.Fatal(err)
	}

	var session *http.Cookie
	t.Run("login sets the session cookie", func(t *testing.T) {
		rec := postJSON(t, app, "/api/auth/login", `{"email": "Admin@loadout.bg", "password": "`+testPassword+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var u User
		if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
		if u.Email != "admin@loadout.bg" || u.Role != RoleAdmin {
			t.Errorf("user %+v", u)
		}
		session = sessionCookie(t, rec)
		// Max-Age counts from when the cookie is written, a moment after the session started.
		if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" ||
			session.MaxAge < 12*60*60-5 || session.MaxAge > 12*60*60 || session.Value == "" {
			t.Errorf("cookie %+v", session)
		}
	})

	t.Run("me", func(t *testing.T) {
		rec := get(t, app, "/api/auth/me", session)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"admin@loadout.bg"`) {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
		for _, cookies := range [][]*http.Cookie{nil, {{Name: "session", Value: "forged"}}} {
			if rec := get(t, app, "/api/auth/me", cookies...); rec.Code != http.StatusUnauthorized ||
				!strings.Contains(rec.Body.String(), `"unauthenticated"`) {
				t.Errorf("cookies %v: status %d: %s", cookies, rec.Code, rec.Body)
			}
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		rec := postJSON(t, app, "/api/auth/login", `{"email": "admin@loadout.bg", "password": "wrong password"}`)
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"invalid_credentials"`) {
			t.Errorf("status %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("too many failures", func(t *testing.T) {
		body := `{"email": "locked@loadout.bg", "password": "wrong password"}`
		for range 6 {
			postJSON(t, app, "/api/auth/login", body)
		}
		rec := postJSON(t, app, "/api/auth/login", body)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "30" ||
			!strings.Contains(rec.Body.String(), `"too_many_attempts"`) {
			t.Errorf("status %d, Retry-After %q: %s", rec.Code, rec.Header().Get("Retry-After"), rec.Body)
		}
	})

	t.Run("logout", func(t *testing.T) {
		rec := post(t, app, "/api/auth/logout", session)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if c := sessionCookie(t, rec); c.MaxAge >= 0 {
			t.Errorf("cookie not cleared: %+v", c)
		}
		if rec := get(t, app, "/api/auth/me", session); rec.Code != http.StatusUnauthorized {
			t.Errorf("me after logout: status %d", rec.Code)
		}
	})
}

func TestProductionSessionCookie(t *testing.T) {
	app := newTestApp()
	app.cfg.Env = config.EnvProduction
	rec := httptest.NewRecorder()
	app.setSessionCookie(rec, "token", time.Now().Add(time.Hour))
	got := rec.Header().Get("Set-Cookie")
	if !strings.HasPrefix(got, "__Host-session=token;") || !strings.Contains(got, "; Secure") ||
		strings.Contains(got, "Domain=") || !strings.Contains(got, "Path=/") {
		t.Errorf("Set-Cookie %q", got)
	}
}

func TestAdminRoutesNeedAnAdmin(t *testing.T) {
	app := newAuthApp(t)
	ctx := context.Background()

	admin := adminCookie(t, app)
	// Customers can't sign up yet, so add one by hand, with the admin's password hash.
	if _, err := app.db.Exec(ctx, `
		INSERT INTO auth.users (email, password_hash, role)
		SELECT 'customer@loadout.bg', password_hash, 'customer' FROM auth.users LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	sess, err := app.auth.Login(ctx, auth.LoginInput{Email: "customer@loadout.bg", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	customer := &http.Cookie{Name: "session", Value: sess.Token}

	const target = "/api/admin/specs/review-queue?category=cpu"
	tests := []struct {
		name    string
		cookies []*http.Cookie
		status  int
		code    string
	}{
		{"no session", nil, http.StatusUnauthorized, "unauthenticated"},
		{"customer", []*http.Cookie{customer}, http.StatusForbidden, "forbidden"},
		{"admin", []*http.Cookie{admin}, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, app, target, tt.cookies...)
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), tt.code) {
				t.Errorf("status %d, want %d %s: %s", rec.Code, tt.status, tt.code, rec.Body)
			}
		})
	}
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	app := newAuthApp(t)
	login := func(header, value string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
			strings.NewReader(`{"email": "a@loadout.bg", "password": "wrong password"}`))
		req.Header.Set(header, value)
		app.routes().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := login("Sec-Fetch-Site", "cross-site"); code != http.StatusForbidden {
		t.Errorf("from another site: status %d, want 403", code)
	}
	if code := login("Origin", "https://evil.example"); code != http.StatusForbidden {
		t.Errorf("other Origin: status %d, want 403", code)
	}
	// Same site, and no browser at all (no headers), reach the handler.
	if code := login("Sec-Fetch-Site", "same-origin"); code != http.StatusUnauthorized {
		t.Errorf("same origin: status %d, want 401", code)
	}
	if code := login("X-Nothing", ""); code != http.StatusUnauthorized {
		t.Errorf("no browser: status %d, want 401", code)
	}
}
