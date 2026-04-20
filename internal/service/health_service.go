package service

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	// HealthStatusOK marks a healthy dependency or aggregate runtime status.
	HealthStatusOK = "ok"
	// HealthStatusDegraded marks a missing or failing dependency.
	HealthStatusDegraded = "degraded"
)

// ComponentProbe describes how to check a runtime dependency.
type ComponentProbe struct {
	Check          func(context.Context) error
	DegradedReason string
}

// RuntimeProbes groups the dependency checks reported by /health.
type RuntimeProbes struct {
	Postgres ComponentProbe
	Minio    ComponentProbe
	RabbitMQ ComponentProbe
}

// HealthSnapshot is the JSON payload returned by the health endpoint.
type HealthSnapshot struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

// RuntimeHealthChecker returns a snapshot of current dependency health.
type RuntimeHealthChecker interface {
	Check(context.Context) HealthSnapshot
}

// RuntimeHealthService evaluates runtime dependency probes with a timeout.
type RuntimeHealthService struct {
	logger  *slog.Logger
	timeout time.Duration
	probes  RuntimeProbes
}

// NewRuntimeHealthService builds a RuntimeHealthService from configured probes.
func NewRuntimeHealthService(logger *slog.Logger, timeout time.Duration, probes RuntimeProbes) *RuntimeHealthService {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &RuntimeHealthService{
		logger:  logger,
		timeout: timeout,
		probes:  probes,
	}
}

// DegradedComponent returns a probe that is always reported as degraded.
func DegradedComponent(reason string) ComponentProbe {
	return ComponentProbe{DegradedReason: reason}
}

// HealthyComponent wraps a dependency health-check function into a probe.
func HealthyComponent(check func(context.Context) error) ComponentProbe {
	return ComponentProbe{Check: check}
}

// Check runs all configured probes and returns the aggregate health snapshot.
func (s *RuntimeHealthService) Check(ctx context.Context) HealthSnapshot {
	components := map[string]string{
		"postgres": s.checkComponent(ctx, "postgres", s.probes.Postgres),
		"minio":    s.checkComponent(ctx, "minio", s.probes.Minio),
		"rabbitmq": s.checkComponent(ctx, "rabbitmq", s.probes.RabbitMQ),
	}
	status := HealthStatusOK
	for _, componentStatus := range components {
		if componentStatus != HealthStatusOK {
			status = HealthStatusDegraded
			break
		}
	}
	return HealthSnapshot{
		Status:     status,
		Components: components,
	}
}

func (s *RuntimeHealthService) checkComponent(ctx context.Context, name string, probe ComponentProbe) string {
	if probe.Check == nil {
		reason := probe.DegradedReason
		if reason == "" {
			reason = "health check is not configured"
		}
		s.logger.Warn("health component degraded", "component", name, "reason", reason)
		return HealthStatusDegraded
	}

	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if err := probe.Check(checkCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
			s.logger.Warn("health component timeout", "component", name, "timeout", s.timeout.String())
			return HealthStatusDegraded
		}
		s.logger.Warn("health component check failed", "component", name, "error", err)
		return HealthStatusDegraded
	}

	return HealthStatusOK
}
