package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// routes builds the router. Middleware reads outside-in: requestID, then per group logRequest,
// then recoverPanic. logRequest sits outside recoverPanic so a request that panics still gets its
// access log line, with status 500.
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
		r.Use(app.logRequest(slog.LevelInfo), app.recoverPanic)
		r.NotFound(app.notFound)
		r.Get("/healthz", app.healthz)

		// TODO(phase-4): move onto the api/openapi.yaml contract (oapi-codegen).
		r.Get("/products", app.listProducts)
		r.Get("/products/{slug}", app.showProduct)
	})

	return r
}
