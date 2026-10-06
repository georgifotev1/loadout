package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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
	app.auth = newAuthService(t, app.db)
	app.specs = specs.NewService(app.db)
	return app
}

func newAuthService(t *testing.T, pool *pgxpool.Pool) *auth.Service {
	t.Helper()
	svc, err := auth.NewService(pool, make([]byte, auth.TOTPKeyLen))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// adminCookie creates an admin with a new email, sets up their TOTP and returns a verified
// session cookie for them.
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
	setup, err := app.auth.EnrollTOTP(ctx, sess.Token)
	if err != nil {
		t.Fatal(err)
	}
	sess, _, err = app.auth.ConfirmTOTP(ctx, sess.Token, totpCode(t, setup.Secret, 0))
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: app.sessionCookieName(), Value: sess.Token}
}

// totpCode is what an authenticator app shows for the base32 secret, steps 30-second steps from
// now (RFC 6238; internal/auth tests it against the RFC's vectors).
func totpCode(t *testing.T, secret string, steps int64) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(time.Now().Unix()/30+steps))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[off:off+4])&0x7fffffff)%1_000_000)
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
		var me Me
		if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
			t.Fatal(err)
		}
		if me.Email != "admin@loadout.bg" || me.Role != RoleAdmin || me.SecondFactor != SecondFactorEnroll {
			t.Errorf("me %+v", me)
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
	if _, err := app.auth.CreateAdmin(ctx, "new-admin@loadout.bg", testPassword); err != nil {
		t.Fatal(err)
	}
	sess, err = app.auth.Login(ctx, auth.LoginInput{Email: "new-admin@loadout.bg", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	passwordOnly := &http.Cookie{Name: "session", Value: sess.Token}

	const target = "/api/admin/specs/review-queue?category=cpu"
	tests := []struct {
		name    string
		cookies []*http.Cookie
		status  int
		code    string
	}{
		{"no session", nil, http.StatusUnauthorized, "unauthenticated"},
		{"customer", []*http.Cookie{customer}, http.StatusForbidden, `"forbidden"`},
		{"admin without the second factor", []*http.Cookie{passwordOnly}, http.StatusForbidden, "second_factor_required"},
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

	t.Run("every admin path, generated or not", func(t *testing.T) {
		for _, tt := range []struct{ method, target string }{
			{http.MethodPatch, "/api/admin/specs/6f1c8e1e-4a3b-4f0e-9a51-2a8d3c1b7e90"},
			{http.MethodPost, "/api/admin/sync"},
			{http.MethodGet, "/api/admin/no-such-page"},
		} {
			if rec := send(t, app, tt.method, tt.target, `{}`, customer); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s: status %d, want 403", tt.method, tt.target, rec.Code)
			}
		}
	})
}

func TestSecondFactorAPI(t *testing.T) {
	app := newAuthApp(t)
	ctx := context.Background()
	if _, err := app.auth.CreateAdmin(ctx, "admin@loadout.bg", testPassword); err != nil {
		t.Fatal(err)
	}
	const adminRoute = "/api/admin/specs/review-queue?category=cpu"
	login := func() *http.Cookie {
		t.Helper()
		rec := postJSON(t, app, "/api/auth/login", `{"email": "admin@loadout.bg", "password": "`+testPassword+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("login: status %d: %s", rec.Code, rec.Body)
		}
		return sessionCookie(t, rec)
	}
	send := func(target, body string, c *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(c)
		app.routes().ServeHTTP(rec, req)
		return rec
	}
	expect := func(rec *httptest.ResponseRecorder, status int, body string) {
		t.Helper()
		if rec.Code != status || !strings.Contains(rec.Body.String(), body) {
			t.Errorf("status %d, want %d with %s: %s", rec.Code, status, body, rec.Body)
		}
	}

	var recovery []string
	t.Run("enrol", func(t *testing.T) {
		c := login()
		expect(get(t, app, "/api/auth/me", c), http.StatusOK, `"second_factor":"enroll"`)
		expect(get(t, app, adminRoute, c), http.StatusForbidden, `"second_factor_required"`)

		rec := post(t, app, "/api/auth/totp/enroll", c)
		var setup TotpSetup
		if err := json.Unmarshal(rec.Body.Bytes(), &setup); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("enroll: status %d: %s", rec.Code, rec.Body)
		}
		if !strings.HasPrefix(setup.Uri, "otpauth://totp/Loadout:admin@loadout.bg?") {
			t.Errorf("uri %s", setup.Uri)
		}

		expect(send("/api/auth/totp/confirm", `{"code": "abc"}`, c), http.StatusUnauthorized, `"invalid_code"`)
		rec = send("/api/auth/totp/confirm", `{"code": "`+totpCode(t, setup.Secret, 0)+`"}`, c)
		var confirmed TotpConfirmed
		if err := json.Unmarshal(rec.Body.Bytes(), &confirmed); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("confirm: status %d: %s", rec.Code, rec.Body)
		}
		if confirmed.Me.SecondFactor != SecondFactorVerified || len(confirmed.RecoveryCodes) != 10 {
			t.Errorf("confirmed %+v", confirmed)
		}
		verified := sessionCookie(t, rec)
		expect(get(t, app, adminRoute, verified), http.StatusOK, "")
		expect(get(t, app, "/api/auth/me", c), http.StatusUnauthorized, `"unauthenticated"`)
		expect(post(t, app, "/api/auth/totp/enroll", verified), http.StatusConflict, `"second_factor_state"`)
		recovery = confirmed.RecoveryCodes

		rec = post(t, app, "/api/auth/totp/recovery-codes", verified)
		var codes RecoveryCodes
		if err := json.Unmarshal(rec.Body.Bytes(), &codes); rec.Code != http.StatusOK || err != nil || len(codes.RecoveryCodes) != 10 {
			t.Fatalf("regenerate: status %d: %s", rec.Code, rec.Body)
		}
		recovery = codes.RecoveryCodes
	})

	t.Run("verify with a recovery code", func(t *testing.T) {
		c := login()
		expect(get(t, app, "/api/auth/me", c), http.StatusOK, `"second_factor":"verify"`)
		expect(post(t, app, "/api/auth/totp/enroll", c), http.StatusConflict, `"second_factor_state"`)
		expect(send("/api/auth/totp/verify", `{"code": "000000"}`, c), http.StatusUnauthorized, `"invalid_code"`)
		rec := send("/api/auth/totp/verify", `{"code": "`+recovery[0]+`"}`, c)
		expect(rec, http.StatusOK, `"second_factor":"verified"`)
		expect(get(t, app, adminRoute, sessionCookie(t, rec)), http.StatusOK, "")
	})

	t.Run("development bypass", func(t *testing.T) {
		app.cfg.Auth.SkipTOTP = true
		defer func() { app.cfg.Auth.SkipTOTP = false }()
		c := login()
		expect(get(t, app, "/api/auth/me", c), http.StatusOK, `"second_factor":"not_required"`)
		expect(get(t, app, adminRoute, c), http.StatusOK, "")
	})
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
