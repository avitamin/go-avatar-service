package observability

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type requestIDKey struct{}

// WithRequestID stores a request/correlation identifier in context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the request/correlation identifier from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// NewLogger builds a JSON slog logger with service and component fields.
func NewLogger(serviceName, component string, out io.Writer) *slog.Logger {
	if serviceName == "" {
		serviceName = "avatar-service"
	}
	if out == nil {
		out = os.Stdout
	}
	attrs := []slog.Attr{slog.String("service", serviceName)}
	if component != "" {
		attrs = append(attrs, slog.String("component", component))
	}
	jsonHandler := slog.NewJSONHandler(out, nil).WithAttrs(attrs)
	otelHandler := currentOTELLogHandler()
	if otelHandler != nil {
		otelHandler = otelHandler.WithAttrs(attrs)
	}
	return slog.New(newFanoutHandler(jsonHandler, otelHandler))
}

// Attrs returns correlation attrs followed by caller-provided key/value pairs.
func Attrs(ctx context.Context, args ...any) []any {
	attrs := make([]any, 0, 6+len(args))
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		attrs = append(attrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	if requestID := RequestID(ctx); requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	return append(attrs, args...)
}
