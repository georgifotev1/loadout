package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// application implements the handlers generated from api/openapi.yaml.
var _ ServerInterface = (*application)(nil)

// routes builds the router. Middleware reads outside-in: requestID, then per group logRequest,
// then recoverPanic. logRequest sits outside recoverPanic so a request that panics still gets its
// access log line, with status 500. Under /api, crossOrigin rejects cross-site writes before
// authenticate looks up the session.
func (app *application) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(app.requestID)

	// Probes live outside /api: they're for the platform, not for clients. Their hits are logged
	// at debug level so they don't drown everything else.
	r.Group(func(r chi.Router) {
		r.Use(app.logRequest(slog.LevelDebug), app.recoverPanic)
		r.Get("/livez", app.livez)
		r.Get("/readyz", app.readyz)
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(app.logRequest(slog.LevelInfo), app.recoverPanic, app.crossOrigin, app.authenticate)
		r.NotFound(app.notFound)

		// The routes in api/openapi.yaml. The generated wrapper binds parameters and calls the
		// ServerInterface methods on app.
		HandlerWithOptions(app, ChiServerOptions{
			BaseRouter:       r,
			ErrorHandlerFunc: app.handleError,
		})

		// TODO(phase-6): move onto api/openapi.yaml with the admin UI that uses them.
		r.Route("/admin", func(r chi.Router) {
			r.Use(app.requireAdmin)
			r.Get("/specs/review-queue", app.specsReviewQueue)
			r.Post("/sync", app.triggerSync)
			r.Get("/jobs/discarded", app.discardedJobs)
		})
	})

	return r
}
