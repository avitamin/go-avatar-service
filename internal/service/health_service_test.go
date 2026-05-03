package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRuntimeHealthServiceCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		probes       RuntimeProbes
		wantStatus   string
		wantPostgres string
		wantMinio    string
		wantRabbitMQ string
	}{
		{
			name: "all healthy",
			probes: RuntimeProbes{
				Postgres: HealthyComponent(func(context.Context) error { return nil }),
				Minio:    HealthyComponent(func(context.Context) error { return nil }),
				RabbitMQ: HealthyComponent(func(context.Context) error { return nil }),
			},
			wantStatus:   HealthStatusOK,
			wantPostgres: HealthStatusOK,
			wantMinio:    HealthStatusOK,
			wantRabbitMQ: HealthStatusOK,
		},
		{
			name: "postgres degraded",
			probes: RuntimeProbes{
				Postgres: HealthyComponent(func(context.Context) error { return errors.New("postgres down") }),
				Minio:    HealthyComponent(func(context.Context) error { return nil }),
				RabbitMQ: HealthyComponent(func(context.Context) error { return nil }),
			},
			wantStatus:   HealthStatusDegraded,
			wantPostgres: HealthStatusDegraded,
			wantMinio:    HealthStatusOK,
			wantRabbitMQ: HealthStatusOK,
		},
		{
			name: "fallback probes degrade immediately",
			probes: RuntimeProbes{
				Postgres: DegradedComponent("memory repository"),
				Minio:    DegradedComponent("memory storage"),
				RabbitMQ: DegradedComponent("log broker"),
			},
			wantStatus:   HealthStatusDegraded,
			wantPostgres: HealthStatusDegraded,
			wantMinio:    HealthStatusDegraded,
			wantRabbitMQ: HealthStatusDegraded,
		},
		{
			name: "timeout degrades component",
			probes: RuntimeProbes{
				Postgres: HealthyComponent(func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				}),
				Minio:    HealthyComponent(func(context.Context) error { return nil }),
				RabbitMQ: HealthyComponent(func(context.Context) error { return nil }),
			},
			wantStatus:   HealthStatusDegraded,
			wantPostgres: HealthStatusDegraded,
			wantMinio:    HealthStatusOK,
			wantRabbitMQ: HealthStatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewRuntimeHealthService(testHealthLogger(), 10*time.Millisecond, tt.probes)
			snapshot := svc.Check(context.Background())

			if snapshot.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", snapshot.Status, tt.wantStatus)
			}
			if snapshot.Components["postgres"] != tt.wantPostgres {
				t.Fatalf("postgres = %q, want %q", snapshot.Components["postgres"], tt.wantPostgres)
			}
			if snapshot.Components["minio"] != tt.wantMinio {
				t.Fatalf("minio = %q, want %q", snapshot.Components["minio"], tt.wantMinio)
			}
			if snapshot.Components["rabbitmq"] != tt.wantRabbitMQ {
				t.Fatalf("rabbitmq = %q, want %q", snapshot.Components["rabbitmq"], tt.wantRabbitMQ)
			}
		})
	}
}

func testHealthLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
