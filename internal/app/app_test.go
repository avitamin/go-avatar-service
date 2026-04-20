package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestCLIContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "server", args: []string{"avatars-service", "server", "--check"}},
		{name: "worker", args: []string{"avatars-service", "worker", "--check"}},
		{name: "migrate up", args: []string{"avatars-service", "migrate", "up", "--check"}},
		{name: "migrate down", args: []string{"avatars-service", "migrate", "down", "--check"}},
		{name: "migrate status", args: []string{"avatars-service", "migrate", "status", "--check"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Run(tt.args, &bytes.Buffer{}); err != nil {
				t.Fatalf("Run(%v) error = %v", tt.args, err)
			}
		})
	}
}

func TestCLIDoesNotRunMigrationsOnServerOrWorker(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"avatars-service", "server", "--check"}, &out); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("migrate")) {
		t.Fatalf("server output mentions migrations: %s", out.String())
	}
	out.Reset()
	if err := Run([]string{"avatars-service", "worker", "--check"}, &out); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("migrate")) {
		t.Fatalf("worker output mentions migrations: %s", out.String())
	}
}

func TestRunServerUsesSignalContext(t *testing.T) {
	origSignalContext := signalContextFn
	origRunServer := runServerFn
	t.Cleanup(func() {
		signalContextFn = origSignalContext
		runServerFn = origRunServer
	})

	type ctxKey string

	wantCtx := context.WithValue(context.Background(), ctxKey("source"), "signal")
	stopCalled := false
	runCalled := false
	signalContextFn = func() (context.Context, context.CancelFunc) {
		return wantCtx, func() { stopCalled = true }
	}
	runServerFn = func(ctx context.Context) error {
		runCalled = true
		if got := ctx.Value(ctxKey("source")); got != "signal" {
			t.Fatalf("Run(server) context source = %v, want signal", got)
		}
		return nil
	}

	if err := Run([]string{"avatars-service", "server"}, io.Discard); err != nil {
		t.Fatalf("Run(server) error = %v", err)
	}
	if !runCalled {
		t.Fatal("Run(server) did not call server runner")
	}
	if !stopCalled {
		t.Fatal("Run(server) did not stop signal context")
	}
}

func TestRunServerGracefulShutdownOnContextCancel(t *testing.T) {
	origNewHTTPServer := newHTTPServer
	t.Cleanup(func() { newHTTPServer = origNewHTTPServer })

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	server := &stubHTTPServer{
		listenAndServe: func() error {
			<-stopped
			return http.ErrServerClosed
		},
		shutdown: func(shutdownCtx context.Context) error {
			close(stopped)
			return nil
		},
	}
	newHTTPServer = func(string, http.Handler) httpServer {
		return server
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunServer(ctx)
	}()

	select {
	case <-time.After(50 * time.Millisecond):
	case err := <-errCh:
		t.Fatalf("RunServer returned before cancellation: %v", err)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunServer() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunServer did not return after context cancellation")
	}

	if !server.shutdownCalled {
		t.Fatal("RunServer did not call Shutdown on context cancellation")
	}
	if server.shutdownCtx == nil {
		t.Fatal("RunServer did not pass shutdown context")
	}
	if _, ok := server.shutdownCtx.Deadline(); !ok {
		t.Fatal("RunServer shutdown context has no deadline")
	}
	if server.closeCalled {
		t.Fatal("RunServer called Close during graceful shutdown")
	}
}

func TestRunServerReturnsShutdownError(t *testing.T) {
	origNewHTTPServer := newHTTPServer
	t.Cleanup(func() { newHTTPServer = origNewHTTPServer })

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	shutdownErr := errors.New("shutdown failed")
	server := &stubHTTPServer{
		listenAndServe: func() error {
			<-stopped
			return http.ErrServerClosed
		},
		shutdown: func(context.Context) error {
			close(stopped)
			return shutdownErr
		},
	}
	newHTTPServer = func(string, http.Handler) httpServer {
		return server
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunServer(ctx)
	}()

	select {
	case <-time.After(50 * time.Millisecond):
	case err := <-errCh:
		t.Fatalf("RunServer returned before cancellation: %v", err)
	}

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, shutdownErr) {
			t.Fatalf("RunServer() error = %v, want shutdown error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunServer did not return after context cancellation")
	}
}

type stubHTTPServer struct {
	listenAndServe func() error
	shutdown       func(context.Context) error
	close          func() error

	shutdownCalled bool
	closeCalled    bool
	shutdownCtx    context.Context
}

func (s *stubHTTPServer) ListenAndServe() error {
	if s.listenAndServe != nil {
		return s.listenAndServe()
	}
	return nil
}

func (s *stubHTTPServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled = true
	s.shutdownCtx = ctx
	if s.shutdown != nil {
		return s.shutdown(ctx)
	}
	return nil
}

func (s *stubHTTPServer) Close() error {
	s.closeCalled = true
	if s.close != nil {
		return s.close()
	}
	return nil
}
