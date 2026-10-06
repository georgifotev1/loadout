package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/platform/config"
)

// Login handles POST /api/auth/login.
func (app *application) Login(w http.ResponseWriter, r *http.Request) {
	var in LoginRequest
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	sess, err := app.auth.Login(r.Context(), auth.LoginInput{
		Email:    in.Email,
		Password: in.Password,
		IP:       clientIP(r),
		OldToken: app.sessionToken(r),
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	app.writeJSON(w, r, http.StatusOK, app.me(sess))
}

// Logout handles POST /api/auth/logout.
func (app *application) Logout(w http.ResponseWriter, r *http.Request) {
	if err := app.auth.Logout(r.Context(), app.sessionToken(r)); err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setSessionCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

// GetMe handles GET /api/auth/me.
func (app *application) GetMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		app.handleError(w, r, auth.ErrUnauthenticated)
		return
	}
	app.writeJSON(w, r, http.StatusOK, app.me(sess))
}

// EnrollTotp handles POST /api/auth/totp/enroll.
func (app *application) EnrollTotp(w http.ResponseWriter, r *http.Request) {
	setup, err := app.auth.EnrollTOTP(r.Context(), app.sessionToken(r))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, TotpSetup{Secret: setup.Secret, Uri: setup.URI})
}

// ConfirmTotp handles POST /api/auth/totp/confirm.
func (app *application) ConfirmTotp(w http.ResponseWriter, r *http.Request) {
	var in CodeRequest
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	sess, codes, err := app.auth.ConfirmTOTP(r.Context(), app.sessionToken(r), in.Code)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	app.writeJSON(w, r, http.StatusOK, TotpConfirmed{Me: app.me(sess), RecoveryCodes: codes})
}

// VerifyTotp handles POST /api/auth/totp/verify.
func (app *application) VerifyTotp(w http.ResponseWriter, r *http.Request) {
	var in CodeRequest
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	sess, err := app.auth.VerifySecondFactor(r.Context(), app.sessionToken(r), in.Code)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	app.writeJSON(w, r, http.StatusOK, app.me(sess))
}

// RegenerateRecoveryCodes handles POST /api/auth/totp/recovery-codes.
func (app *application) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := app.auth.RegenerateRecoveryCodes(r.Context(), app.sessionToken(r))
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, RecoveryCodes{RecoveryCodes: codes})
}

// me reports an admin's second factor as not required when AUTH_SKIP_TOTP is on, so the web app
// doesn't ask for a code requireAdmin won't check.
func (app *application) me(sess auth.Session) Me {
	sf := SecondFactor(sess.SecondFactor)
	if app.cfg.Auth.SkipTOTP {
		sf = SecondFactorNotRequired
	}
	u := sess.User
	return Me{Id: u.ID, Email: u.Email, Role: Role(u.Role), SecondFactor: sf}
}

// sessionToken is the session cookie's value, or "" without one.
func (app *application) sessionToken(r *http.Request) string {
	if c, err := r.Cookie(app.sessionCookieName()); err == nil {
		return c.Value
	}
	return ""
}

// sessionCookieName has the __Host- prefix in production: the browser then only accepts the
// cookie over HTTPS, for this exact host and path /, so a subdomain can't set or overwrite it.
// Locally the API runs on plain http, where that cookie wouldn't be stored.
func (app *application) sessionCookieName() string {
	if app.cfg.Env == config.EnvProduction {
		return "__Host-session"
	}
	return "session"
}

// setSessionCookie sets the session cookie, or deletes it when token is empty. HttpOnly keeps it
// away from JavaScript (and so from XSS), and SameSite=Lax keeps other sites' forms and fetches
// from sending it.
func (app *application) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	c := &http.Cookie{
		Name:     app.sessionCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   app.cfg.Env == config.EnvProduction,
		SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires
		c.MaxAge = int(time.Until(expires).Round(time.Second).Seconds())
	}
	http.SetCookie(w, c)
}

func sessionFrom(ctx context.Context) (auth.Session, bool) {
	sess, ok := ctx.Value(sessionKey).(auth.Session)
	return sess, ok
}

// clientIP is the address the request came from, for the per-IP login limit.
// TODO(phase-8): behind the load balancer this is the balancer; read the client from
// X-Forwarded-For then, once the routing is decided.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
