// Package observability contains runtime logging, tracing, and metrics wiring.
package observability

import (
	"os"
	"strconv"
)

// Config contains environment-driven observability settings.
type Config struct {
	ServiceName      string
	ServiceVersion   string
	TracesEnabled    bool
	LogsEnabled      bool
	OTLPEndpoint     string
	OTLPInsecure     bool
	OTLPLogsEndpoint string
	OTLPLogsInsecure bool
	MetricsAddr      string
}

// ConfigFromEnv reads observability configuration from process environment.
func ConfigFromEnv() Config {
	cfg := Config{
		ServiceName:      os.Getenv("SERVICE_NAME"),
		ServiceVersion:   os.Getenv("SERVICE_VERSION"),
		TracesEnabled:    true,
		LogsEnabled:      false,
		OTLPEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTLPInsecure:     parseBoolEnv("OTEL_EXPORTER_OTLP_INSECURE", false),
		OTLPLogsEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"),
		MetricsAddr:      os.Getenv("METRICS_ADDR"),
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "avatar-service"
	}
	if raw := os.Getenv("OTEL_TRACES_ENABLED"); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.TracesEnabled = enabled
		}
	}
	if raw := os.Getenv("OTEL_LOGS_ENABLED"); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.LogsEnabled = enabled
		}
	}
	cfg.OTLPLogsInsecure = cfg.OTLPInsecure
	if raw := os.Getenv("OTEL_EXPORTER_OTLP_LOGS_INSECURE"); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.OTLPLogsInsecure = enabled
		}
	}
	if cfg.OTLPLogsEndpoint == "" {
		cfg.OTLPLogsEndpoint = cfg.OTLPEndpoint
	}
	return cfg
}

func parseBoolEnv(key string, fallback bool) bool {
	if raw := os.Getenv(key); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			return enabled
		}
	}
	return fallback
}
