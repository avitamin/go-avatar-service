package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go-avatar-service/internal/observability"
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

func TestRateLimitConfigFromEnvMapsTrustForwardedHeaders(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv("RATE_LIMIT_TRUST_FORWARDED_HEADERS", "true")

	cfg, err := rateLimitConfigFromEnv()
	if err != nil {
		t.Fatalf("rateLimitConfigFromEnv() error = %v", err)
	}
	if !cfg.TrustForwardedHeaders {
		t.Fatal("TrustForwardedHeaders = false, want true")
	}
}

func TestEnvParsersUseFallbackOnlyWhenUnset(t *testing.T) {
	unsetenv(t, "TEST_BOOL_ENV")
	unsetenv(t, "TEST_FLOAT_ENV")
	unsetenv(t, "TEST_INT_ENV")

	boolValue, err := parseBoolEnvDefault("TEST_BOOL_ENV", true)
	if err != nil {
		t.Fatalf("parseBoolEnvDefault() error = %v", err)
	}
	if !boolValue {
		t.Fatal("bool fallback = false, want true")
	}

	floatValue, err := parseFloatEnvDefault("TEST_FLOAT_ENV", 1.5)
	if err != nil {
		t.Fatalf("parseFloatEnvDefault() error = %v", err)
	}
	if floatValue != 1.5 {
		t.Fatalf("float fallback = %v, want 1.5", floatValue)
	}

	intValue, err := parseIntEnvDefault("TEST_INT_ENV", 7)
	if err != nil {
		t.Fatalf("parseIntEnvDefault() error = %v", err)
	}
	if intValue != 7 {
		t.Fatalf("int fallback = %v, want 7", intValue)
	}
}

func TestEnvParsersRejectExplicitEmptyValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		parse func(string) error
	}{
		{
			name: "bool",
			key:  "TEST_EMPTY_BOOL_ENV",
			parse: func(key string) error {
				_, err := parseBoolEnvDefault(key, true)
				return err
			},
		},
		{
			name: "float",
			key:  "TEST_EMPTY_FLOAT_ENV",
			parse: func(key string) error {
				_, err := parseFloatEnvDefault(key, 1)
				return err
			},
		},
		{
			name: "int",
			key:  "TEST_EMPTY_INT_ENV",
			parse: func(key string) error {
				_, err := parseIntEnvDefault(key, 1)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, "")

			err := tt.parse(tt.key)

			if err == nil {
				t.Fatal("error = nil, want explicit empty value error")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error = %v, want env var name %q", err, tt.key)
			}
		})
	}
}

func TestEnvParsersRejectMalformedValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		parse func(string) error
	}{
		{
			name:  "bool",
			key:   "RATE_LIMIT_ENABLED",
			value: "tru",
			parse: func(key string) error {
				_, err := parseBoolEnvDefault(key, true)
				return err
			},
		},
		{
			name:  "float",
			key:   "RATE_LIMIT_REQUESTS_PER_SECOND",
			value: "fast",
			parse: func(key string) error {
				_, err := parseFloatEnvDefault(key, 20)
				return err
			},
		},
		{
			name:  "int",
			key:   "RATE_LIMIT_BURST",
			value: "many",
			parse: func(key string) error {
				_, err := parseIntEnvDefault(key, 40)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)

			err := tt.parse(tt.key)

			if err == nil {
				t.Fatal("error = nil, want malformed value error")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error = %v, want env var name %q", err, tt.key)
			}
		})
	}
}

func TestNumericEnvParsersRejectNonPositiveValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		parse func(string) error
	}{
		{
			name:  "float zero",
			key:   "RATE_LIMIT_REQUESTS_PER_SECOND",
			value: "0",
			parse: func(key string) error {
				_, err := parseFloatEnvDefault(key, 20)
				return err
			},
		},
		{
			name:  "float negative",
			key:   "RATE_LIMIT_REQUESTS_PER_SECOND",
			value: "-1",
			parse: func(key string) error {
				_, err := parseFloatEnvDefault(key, 20)
				return err
			},
		},
		{
			name:  "int zero",
			key:   "CIRCUIT_BREAKER_FAILURE_THRESHOLD",
			value: "0",
			parse: func(key string) error {
				_, err := parseIntEnvDefault(key, 5)
				return err
			},
		},
		{
			name:  "int negative",
			key:   "CIRCUIT_BREAKER_OPEN_TIMEOUT_SECONDS",
			value: "-30",
			parse: func(key string) error {
				_, err := parseIntEnvDefault(key, 30)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)

			err := tt.parse(tt.key)

			if err == nil {
				t.Fatal("error = nil, want non-positive value error")
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error = %v, want env var name %q", err, tt.key)
			}
		})
	}
}

func TestRunServerReturnsInvalidRateLimitEnvBeforeStartingServer(t *testing.T) {
	clearRuntimeConfigEnv(t)
	origNewHTTPServer := newHTTPServer
	t.Cleanup(func() { newHTTPServer = origNewHTTPServer })
	newHTTPServer = func(string, http.Handler) httpServer {
		t.Fatal("RunServer started HTTP server with invalid rate limit config")
		return nil
	}
	t.Setenv("RATE_LIMIT_REQUESTS_PER_SECOND", "0")

	err := RunServer(context.Background())

	if err == nil {
		t.Fatal("RunServer() error = nil, want config error")
	}
	if !strings.Contains(err.Error(), "RATE_LIMIT_REQUESTS_PER_SECOND") {
		t.Fatalf("RunServer() error = %v, want RATE_LIMIT_REQUESTS_PER_SECOND", err)
	}
}

func TestRunServerReturnsInvalidCircuitBreakerEnvBeforeStartingServer(t *testing.T) {
	clearRuntimeConfigEnv(t)
	origNewHTTPServer := newHTTPServer
	t.Cleanup(func() { newHTTPServer = origNewHTTPServer })
	newHTTPServer = func(string, http.Handler) httpServer {
		t.Fatal("RunServer started HTTP server with invalid circuit breaker config")
		return nil
	}
	t.Setenv("CIRCUIT_BREAKER_FAILURE_THRESHOLD", "0")

	err := RunServer(context.Background())

	if err == nil {
		t.Fatal("RunServer() error = nil, want config error")
	}
	if !strings.Contains(err.Error(), "CIRCUIT_BREAKER_FAILURE_THRESHOLD") {
		t.Fatalf("RunServer() error = %v, want CIRCUIT_BREAKER_FAILURE_THRESHOLD", err)
	}
}

func TestCheckCommandsIgnoreInvalidRuntimeEnv(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv("RATE_LIMIT_ENABLED", "")
	t.Setenv("CIRCUIT_BREAKER_FAILURE_THRESHOLD", "0")

	for _, args := range [][]string{
		{"avatars-service", "server", "--check"},
		{"avatars-service", "worker", "--check"},
		{"avatars-service", "migrate", "up", "--check"},
	} {
		if err := Run(args, io.Discard); err != nil {
			t.Fatalf("Run(%v) error = %v", args, err)
		}
	}
}

func TestWorkerMonitoringHandlerExposesProcessHealth(t *testing.T) {
	handler := workerMonitoringHandler(observability.NewMetrics(nil))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("/health Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := rec.Body.String(); got != "ok\n" {
		t.Fatalf("/health body = %q, want ok\\n", got)
	}
}

func TestWorkerMonitoringHandlerStillExposesMetrics(t *testing.T) {
	metrics := observability.NewMetrics(nil)
	metrics.ObserveWorkerMessage("avatar.uploaded", "success", time.Millisecond)
	handler := workerMonitoringHandler(metrics)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); !strings.Contains(got, "avatar_worker_messages_total") {
		t.Fatalf("/metrics missing worker metrics:\n%s", got)
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
	clearRuntimeConfigEnv(t)
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
	clearRuntimeConfigEnv(t)
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

func clearRuntimeConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"RATE_LIMIT_ENABLED",
		"RATE_LIMIT_REQUESTS_PER_SECOND",
		"RATE_LIMIT_BURST",
		"RATE_LIMIT_TRUST_FORWARDED_HEADERS",
		"CIRCUIT_BREAKER_ENABLED",
		"CIRCUIT_BREAKER_FAILURE_THRESHOLD",
		"CIRCUIT_BREAKER_OPEN_TIMEOUT_SECONDS",
	} {
		unsetenv(t, key)
	}
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	originalValue, hadOriginalValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if hadOriginalValue {
			_ = os.Setenv(key, originalValue)
			return
		}
		_ = os.Unsetenv(key)
	})
}
