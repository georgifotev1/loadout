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
	var old string
	if c, err := r.Cookie(app.sessionCookieName()); err == nil {
		old = c.Value
	}
	sess, err := app.auth.Login(r.Context(), auth.LoginInput{
		Email:    in.Email,
		Password: in.Password,
		IP:       clientIP(r),
		OldToken: old,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	app.writeJSON(w, r, http.StatusOK, user(sess.User))
}

// Logout handles POST /api/auth/logout.
func (app *application) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(app.sessionCookieName()); err == nil {
		if err := app.auth.Logout(r.Context(), c.Value); err != nil {
			app.handleError(w, r, err)
			return
		}
	}
	app.setSessionCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

// GetMe handles GET /api/auth/me.
func (app *application) GetMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r.Context())
	if !ok {
		app.handleError(w, r, auth.ErrUnauthenticated)
		return
	}
	app.writeJSON(w, r, http.StatusOK, user(u))
}

func user(u auth.User) User {
	return User{Id: u.ID, Email: u.Email, Role: Role(u.Role)}
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

func userFrom(ctx context.Context) (auth.User, bool) {
	u, ok := ctx.Value(userKey).(auth.User)
	return u, ok
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
