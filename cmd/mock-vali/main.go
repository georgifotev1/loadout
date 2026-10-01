//	GET  /v1/products?since=&page=&page_size=   changed products, paginated, sorted by sku
//	GET  /v1/products/{sku}
//	POST /admin/mutate   change a product's price, stock, specs or name (bumps updated_at)
//	POST /admin/reset    reload the fixtures and drop all mutations
//
// Test knobs on /v1: ?latency_ms= and ?fail_rate= per request, or -latency / -fail-rate for
// every request. State is in memory only; restarting resets it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gfotev/loadout/fixtures"
	"github.com/gfotev/loadout/internal/platform/obs"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fset := flag.NewFlagSet("mock-vali", flag.ContinueOnError)
	addr := fset.String("addr", ":8081", "listen address")
	latency := fset.Duration("latency", 0, "delay added to every /v1 request")
	failRate := fset.Float64("fail-rate", 0, "fraction of /v1 requests that fail with 503 (0..1)")
	logLevel := fset.String("log-level", "info", "debug | info | warn | error")
	if err := fset.Parse(args); err != nil {
		return err
	}
	if *failRate < 0 || *failRate > 1 {
		return errors.New("-fail-rate must be between 0 and 1")
	}
	if *latency < 0 || *latency > maxLatency {
		return fmt.Errorf("-latency must be between 0 and %s", maxLatency)
	}

	logger := obs.NewLogger(os.Stderr, *logLevel, "text")
	st, err := newStore(fixtures.Vali, time.Now)
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}
	s := &server{store: st, logger: logger, latency: *latency, failRate: *failRate, rand: rand.Float64}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           s.routes(),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		ReadHeaderTimeout: 5 * time.Second,
		// Room for the largest -latency / ?latency_ms= plus the response itself.
		WriteTimeout: maxLatency + 10*time.Second,
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *addr, err)
	}
	_, total := st.list(time.Time{}, 1, 1)
	logger.Info("mock-vali listening", "addr", ln.Addr().String(), "products", total,
		"latency", *latency, "fail_rate", *failRate)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
