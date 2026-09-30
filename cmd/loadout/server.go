package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type serverConfig struct {
	Addr            string
	Handler         http.Handler
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
	DrainDelay      time.Duration
	BeforeShutdown  func()
}

func runServer(ctx context.Context, cfg serverConfig) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           cfg.Handler,
		ErrorLog:          slog.NewLogLogger(cfg.Logger.Handler(), slog.LevelWarn),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}
	cfg.Logger.Info("http server listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// The server stopped on its own (it never returns ErrServerClosed here, since only we call Shutdown).
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	cfg.Logger.Info("shutdown started", "drain_delay", cfg.DrainDelay, "timeout", cfg.ShutdownTimeout)
	if cfg.BeforeShutdown != nil {
		cfg.BeforeShutdown()
	}
	if cfg.DrainDelay > 0 {
		time.Sleep(cfg.DrainDelay)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	// Shutdown closes the listener, then waits for active requests to complete.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Timed out: force-close whatever is still open.
		_ = srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	cfg.Logger.Info("shutdown complete")
	return nil
}
