package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// An in-flight request that started before shutdown must finish successfully.
func TestRunServerDrainsInFlightRequests(t *testing.T) {
	addr := freeAddr(t)
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-time.After(300 * time.Millisecond):
			_, _ = w.Write([]byte("done"))
		case <-r.Context().Done():
			t.Error("request context was cancelled during graceful shutdown")
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	beforeShutdown := make(chan struct{})
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- runServer(ctx, serverConfig{
			Addr:            addr,
			Handler:         handler,
			Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
			ShutdownTimeout: 2 * time.Second,
			BeforeShutdown:  func() { close(beforeShutdown) },
		})
	}()

	type result struct {
		body string
		err  error
	}
	res := make(chan result, 1)
	go func() {
		var resp *http.Response
		var err error
		for range 50 { // wait for the listener
			if resp, err = http.Get("http://" + addr); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err != nil {
			res <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		res <- result{string(b), err}
	}()

	<-started
	cancel() // simulate SIGTERM mid-request

	r := <-res
	if r.err != nil || r.body != "done" {
		t.Fatalf("in-flight request: body=%q err=%v", r.body, r.err)
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("runServer returned %v, want nil", err)
	}
	select {
	case <-beforeShutdown:
	default:
		t.Error("BeforeShutdown was not called")
	}
}
