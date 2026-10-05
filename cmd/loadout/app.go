package main

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/platform/config"
)

type application struct {
	cfg          config.Config
	logger       *slog.Logger
	db           *pgxpool.Pool
	catalog      *catalog.Service
	shuttingDown atomic.Bool
}

// serve runs the HTTP API until ctx is cancelled, then drains and returns.
func (app *application) serve(ctx context.Context) error {
	return runServer(ctx, serverConfig{
		Addr:            app.cfg.HTTP.Addr,
		Handler:         app.routes(),
		Logger:          app.logger,
		ShutdownTimeout: app.cfg.HTTP.ShutdownTimeout,
		DrainDelay:      app.cfg.HTTP.DrainDelay,
		BeforeShutdown:  func() { app.shuttingDown.Store(true) },
	})
}
