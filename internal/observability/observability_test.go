package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestConfigFromEnvDefaultsAndNoopTracing(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("OTEL_TRACES_ENABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("METRICS_ADDR", "")

	cfg := ConfigFromEnv()
	if cfg.ServiceName != "avatar-service" {
		t.Fatalf("ServiceName = %q, want avatar-service", cfg.ServiceName)
	}
	if !cfg.TracesEnabled {
		t.Fatal("TracesEnabled = false, want true")
	}
	if cfg.OTLPEndpoint != "" {
		t.Fatalf("OTLPEndpoint = %q, want empty", cfg.OTLPEndpoint)
	}

	shutdown, err := InitTracing(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracing() error = %v", err)
	}
	defer shutdown(context.Background())

	tracer := otel.Tracer("test")
	_, span := tracer.Start(context.Background(), "noop-span")
	defer span.End()
	if !span.SpanContext().IsValid() {
		t.Fatal("noop tracing must still create valid local span contexts")
	}
}

func TestLoggerAddsTraceAndRequestFields(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("avatar-service", "worker", &buf)
	ctx := WithRequestID(context.Background(), "req-123")
	ctx, span := trace.NewNoopTracerProvider().Tracer("test").Start(ctx, "operation")
	defer span.End()

	log.InfoContext(ctx, "processed", Attrs(ctx, "avatar_id", "avatar-1")...)

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode log JSON: %v\n%s", err, buf.String())
	}
	if payload["service"] != "avatar-service" || payload["component"] != "worker" {
		t.Fatalf("service/component fields missing: %v", payload)
	}
	if payload["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want req-123", payload["request_id"])
	}
	if payload["trace_id"] == "" || payload["span_id"] == "" {
		t.Fatalf("trace fields missing: %v", payload)
	}
	if payload["avatar_id"] != "avatar-1" {
		t.Fatalf("avatar_id = %v, want avatar-1", payload["avatar_id"])
	}
	if !strings.Contains(buf.String(), `"msg":"processed"`) {
		t.Fatalf("log message missing: %s", buf.String())
	}
}

func TestRequestIDHelpers(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	if got := RequestID(ctx); got != "req-1" {
		t.Fatalf("RequestID() = %q, want req-1", got)
	}
	if got := RequestID(context.Background()); got != "" {
		t.Fatalf("empty RequestID() = %q, want empty", got)
	}
}

func TestNewLoggerDefaultsOutput(t *testing.T) {
	log := NewLogger("", "", nil)
	if log == nil {
		t.Fatal("NewLogger() returned nil")
	}
	log.Handler().Enabled(context.Background(), slog.LevelInfo)
}
