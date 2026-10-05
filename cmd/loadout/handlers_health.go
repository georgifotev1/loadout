package main

import (
	"context"
	"net/http"
	"time"
)

// livez answers "is the process alive?". It checks nothing external: if the DB is down,
// restarting this container won't fix it, so liveness must not depend on it.
func (app *application) livez(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// readyz answers "should this instance receive traffic?". It fails while shutting down and
// when the database is unreachable.
func (app *application) readyz(w http.ResponseWriter, r *http.Request) {
	if app.shuttingDown.Load() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}
	if err := app.pingDB(r.Context()); err != nil {
		app.logger.Warn("readiness: db ping failed", "err", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// GetHealth handles GET /api/healthz, the JSON health summary the web app shows. Unlike the
// probes it's under /api.
func (app *application) GetHealth(w http.ResponseWriter, r *http.Request) {
	resp := Health{
		Status:   HealthStatusOk,
		Version:  version,
		Env:      string(app.cfg.Env),
		Database: HealthDatabaseOk,
		Time:     time.Now().UTC().Truncate(time.Second),
	}
	status := http.StatusOK
	if err := app.pingDB(r.Context()); err != nil {
		app.logger.Warn("healthz: db ping failed", "err", err)
		resp.Status, resp.Database = HealthStatusDegraded, HealthDatabaseUnavailable
		status = http.StatusServiceUnavailable
	}
	app.writeJSON(w, r, status, resp)
}

func (app *application) pingDB(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return app.db.Ping(ctx)
}
