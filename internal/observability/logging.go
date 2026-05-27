package observability

import (
	"context"
	"log/slog"
	"sync"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

var otelLogHandler = struct {
	sync.RWMutex
	handler slog.Handler
}{}

// InitLogging configures the OpenTelemetry logs provider used by NewLogger.
func InitLogging(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	clearOTELLogHandler()
	if !cfg.LogsEnabled {
		provider := sdklog.NewLoggerProvider()
		global.SetLoggerProvider(provider)
		return provider.Shutdown, nil
	}
	res, err := newResource(cfg)
	if err != nil {
		return nil, err
	}
	opts := []sdklog.LoggerProviderOption{sdklog.WithResource(res)}
	if cfg.OTLPLogsEndpoint != "" {
		exporterOpts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.OTLPLogsEndpoint)}
		if cfg.OTLPLogsInsecure {
			exporterOpts = append(exporterOpts, otlploggrpc.WithInsecure())
		}
		exporter, err := otlploggrpc.New(ctx, exporterOpts...)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)))
	}
	provider := sdklog.NewLoggerProvider(opts...)
	global.SetLoggerProvider(provider)
	setOTELLogHandler(otelslog.NewHandler("go-avatar-service", otelslog.WithLoggerProvider(provider)))
	return provider.Shutdown, nil
}

func newResource(cfg Config) (*resource.Resource, error) {
	resAttrs := []attributeOption{
		{key: string(semconv.ServiceNameKey), value: cfg.ServiceName},
	}
	if cfg.ServiceVersion != "" {
		resAttrs = append(resAttrs, attributeOption{key: string(semconv.ServiceVersionKey), value: cfg.ServiceVersion})
	}
	return resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, toAttributes(resAttrs)...))
}

func setOTELLogHandler(handler slog.Handler) {
	otelLogHandler.Lock()
	defer otelLogHandler.Unlock()
	otelLogHandler.handler = handler
}

func clearOTELLogHandler() {
	setOTELLogHandler(nil)
}

func currentOTELLogHandler() slog.Handler {
	otelLogHandler.RLock()
	defer otelLogHandler.RUnlock()
	return otelLogHandler.handler
}

type fanoutHandler struct {
	handlers []slog.Handler
}

func newFanoutHandler(handlers ...slog.Handler) slog.Handler {
	out := make([]slog.Handler, 0, len(handlers))
	for _, handler := range handlers {
		if handler != nil {
			out = append(out, handler)
		}
	}
	if len(out) == 1 {
		return out[0]
	}
	return fanoutHandler{handlers: out}
}

func (h fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (h fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return fanoutHandler{handlers: handlers}
}

func (h fanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return fanoutHandler{handlers: handlers}
}
