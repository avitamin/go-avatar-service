package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestConfigFromEnvDefaultsAndNoopTracing(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("OTEL_TRACES_ENABLED", "")
	t.Setenv("OTEL_LOGS_ENABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_INSECURE", "")
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
	if cfg.LogsEnabled {
		t.Fatal("LogsEnabled = true, want false")
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

func TestConfigFromEnvReadsOTLPLogsSettings(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_LOGS_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "logs-collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_INSECURE", "false")

	cfg := ConfigFromEnv()
	if !cfg.LogsEnabled {
		t.Fatal("LogsEnabled = false, want true")
	}
	if cfg.OTLPLogsEndpoint != "logs-collector:4317" {
		t.Fatalf("OTLPLogsEndpoint = %q, want logs-collector:4317", cfg.OTLPLogsEndpoint)
	}
	if cfg.OTLPLogsInsecure {
		t.Fatal("OTLPLogsInsecure = true, want false")
	}
	if !cfg.OTLPInsecure {
		t.Fatal("OTLPInsecure = false, want true")
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

func TestLoggerWritesJSONAndOpenTelemetryLogRecords(t *testing.T) {
	exporter := &memoryLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	restore := setOTELLogProviderForTest(t, provider)
	defer restore()

	var buf bytes.Buffer
	log := NewLogger("avatar-service", "server", &buf)
	ctx := WithRequestID(context.Background(), "req-otel")
	traceProvider := sdktrace.NewTracerProvider()
	defer traceProvider.Shutdown(context.Background())
	ctx, span := traceProvider.Tracer("test").Start(ctx, "operation")
	defer span.End()

	log.InfoContext(ctx, "uploaded", Attrs(ctx, "avatar_id", "avatar-42")...)
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
	}

	if !strings.Contains(buf.String(), `"msg":"uploaded"`) {
		t.Fatalf("JSON stdout log missing message: %s", buf.String())
	}
	records := exporter.records()
	if len(records) != 1 {
		t.Fatalf("exported records = %d, want 1", len(records))
	}
	got := recordAttributes(records[0])
	if got["service"] != "avatar-service" || got["component"] != "server" {
		t.Fatalf("service/component attrs missing: %#v", got)
	}
	if got["request_id"] != "req-otel" || got["avatar_id"] != "avatar-42" {
		t.Fatalf("correlation attrs missing: %#v", got)
	}
	if got["trace_id"] == "" || got["span_id"] == "" {
		t.Fatalf("trace attrs missing: %#v", got)
	}
	if records[0].Body().String() != "uploaded" {
		t.Fatalf("body = %q, want uploaded", records[0].Body().String())
	}
}

type memoryLogExporter struct {
	mu      sync.Mutex
	entries []sdklog.Record
}

func (e *memoryLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.entries = append(e.entries, record.Clone())
	}
	return nil
}

func (e *memoryLogExporter) Shutdown(context.Context) error {
	return nil
}

func (e *memoryLogExporter) ForceFlush(context.Context) error {
	return nil
}

func (e *memoryLogExporter) records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdklog.Record, len(e.entries))
	copy(out, e.entries)
	return out
}

func setOTELLogProviderForTest(t *testing.T, provider *sdklog.LoggerProvider) func() {
	t.Helper()
	setOTELLogHandler(otelslog.NewHandler("test", otelslog.WithLoggerProvider(provider)))
	return func() {
		clearOTELLogHandler()
		_ = provider.Shutdown(context.Background())
	}
}

func recordAttributes(record sdklog.Record) map[string]string {
	attrs := make(map[string]string)
	record.WalkAttributes(func(kv otellog.KeyValue) bool {
		attrs[kv.Key] = kv.Value.String()
		return true
	})
	return attrs
}
