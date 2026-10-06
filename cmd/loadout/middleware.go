package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gfotev/loadout/internal/auth"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	sessionKey
)

const requestIDHeader = "X-Request-Id"

func (app *application) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	var b [12]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// logRequest returns middleware that writes one access log line per request at the given level.
func (app *application) logRequest(level slog.Level) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			app.logger.LogAttrs(r.Context(), level, "http request",
				slog.String("request_id", requestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Int("bytes", sw.bytes),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// authenticate puts the session of the cookie, if there is a valid one, in the request
// context. It doesn't reject anything: handlers and requireAdmin decide what needs a user.
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(app.sessionCookieName())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := app.auth.Session(r.Context(), c.Value)
		switch {
		case errors.Is(err, auth.ErrUnauthenticated):
			next.ServeHTTP(w, r)
		case err != nil:
			app.handleError(w, r, err)
		default:
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, sess)))
		}
	})
}

// requireAdmin lets only admins who passed the second factor through: 401 without a session,
// 403 forbidden for anyone else, and 403 second_factor_required for an admin who still has to
// enter a TOTP code (or set TOTP up). AUTH_SKIP_TOTP turns the last check off in development.
func (app *application) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		switch {
		case !ok:
			app.handleError(w, r, auth.ErrUnauthenticated)
		case sess.User.Role != auth.RoleAdmin:
			app.handleError(w, r, auth.ErrAdminsOnly)
		case sess.SecondFactor != auth.SecondFactorVerified && !app.cfg.Auth.SkipTOTP:
			app.writeError(w, r, http.StatusForbidden, ErrorCodeSecondFactorRequired, "enter the second factor first")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// crossOrigin rejects state-changing requests (POST, PUT, DELETE, ...) that a browser sends from
// another site: the CSRF check. The browser says where a request comes from in Sec-Fetch-Site
// (or Origin), and a page can't fake it. Requests without either, like curl, aren't from a
// browser and pass.
func (app *application) crossOrigin(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.writeError(w, r, http.StatusForbidden, ErrorCodeForbidden, "cross-origin request rejected")
	}))
	return cop.Handler(next)
}

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p) // the server treats this one specially; let it through
				}
				w.Header().Set("Connection", "close")
				app.serverError(w, r, fmt.Errorf("panic: %v", p))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter records the status code and body size for the access log.
type statusWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status, w.wroteHeader = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer (Flush, deadlines, ...).
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
