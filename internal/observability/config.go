// Package observability contains runtime logging, tracing, and metrics wiring.
package observability

import (
	"os"
	"strconv"
)

// Config contains environment-driven observability settings.
type Config struct {
	ServiceName    string
	ServiceVersion string
	TracesEnabled  bool
	OTLPEndpoint   string
	MetricsAddr    string
}

// ConfigFromEnv reads observability configuration from process environment.
func ConfigFromEnv() Config {
	cfg := Config{
		ServiceName:    os.Getenv("SERVICE_NAME"),
		ServiceVersion: os.Getenv("SERVICE_VERSION"),
		TracesEnabled:  true,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		MetricsAddr:    os.Getenv("METRICS_ADDR"),
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "avatar-service"
	}
	if raw := os.Getenv("OTEL_TRACES_ENABLED"); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.TracesEnabled = enabled
		}
	}
	return cfg
}
