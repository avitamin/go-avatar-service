package service

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	HealthStatusOK       = "ok"
	HealthStatusDegraded = "degraded"
)

type ComponentProbe struct {
	Check          func(context.Context) error
	DegradedReason string
}

type RuntimeProbes struct {
	Postgres ComponentProbe
	Minio    ComponentProbe
	RabbitMQ ComponentProbe
}

type HealthSnapshot struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

type RuntimeHealthChecker interface {
	Check(context.Context) HealthSnapshot
}

type RuntimeHealthService struct {
	logger  *slog.Logger
	timeout time.Duration
	probes  RuntimeProbes
}

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

func DegradedComponent(reason string) ComponentProbe {
	return ComponentProbe{DegradedReason: reason}
}

func HealthyComponent(check func(context.Context) error) ComponentProbe {
	return ComponentProbe{Check: check}
}

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
