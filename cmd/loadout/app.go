package main

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/gfotev/loadout/internal/auth"
	"github.com/gfotev/loadout/internal/carrier"
	"github.com/gfotev/loadout/internal/catalog"
	"github.com/gfotev/loadout/internal/configurator"
	"github.com/gfotev/loadout/internal/orders"
	"github.com/gfotev/loadout/internal/payments"
	"github.com/gfotev/loadout/internal/platform/config"
	"github.com/gfotev/loadout/internal/platform/jobs"
	"github.com/gfotev/loadout/internal/specs"
)

type application struct {
	cfg          config.Config
	logger       *slog.Logger
	db           *pgxpool.Pool
	auth         *auth.Service
	catalog      *catalog.Service
	specs        *specs.Service
	configurator *configurator.Service
	orders       *orders.Service
	payments     *payments.Service // nil without Stripe settings: cash on delivery only
	carrier      carrier.Carrier
	jobs         *jobs.Client
	shuttingDown atomic.Bool
}

// serve runs the HTTP API and the job workers until ctx is cancelled, then drains both and
// returns. If either fails, the other is shut down too: an instance that serves requests but
// silently stopped working jobs (or the other way round) is worse than one that exits.
func (app *application) serve(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return runServer(ctx, serverConfig{
			Addr:            app.cfg.HTTP.Addr,
			Handler:         app.routes(),
			Logger:          app.logger,
			ShutdownTimeout: app.cfg.HTTP.ShutdownTimeout,
			DrainDelay:      app.cfg.HTTP.DrainDelay,
			BeforeShutdown:  func() { app.shuttingDown.Store(true) },
		})
	})
	if app.jobs.Working() {
		g.Go(func() error {
			return app.jobs.Run(ctx)
		})
	}
	return g.Wait()
}
