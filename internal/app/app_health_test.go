package app

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go-avatar-service/internal/service"
)

func TestServerHealthServiceReportsFallbackDependenciesAsDegraded(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("MINIO_ENDPOINT", "")
	t.Setenv("MINIO_ACCESS_KEY", "")
	t.Setenv("MINIO_SECRET_KEY", "")
	t.Setenv("MINIO_BUCKET", "")
	t.Setenv("MINIO_USE_SSL", "")
	t.Setenv("RABBITMQ_URL", "")

	storeRuntime, err := newStoreFromEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer storeRuntime.close()

	brokerRuntime, err := newBrokerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer brokerRuntime.close()

	snapshot := newServerHealthService(testLogger(), storeRuntime, brokerRuntime).Check(context.Background())

	if snapshot.Status != service.HealthStatusDegraded {
		t.Fatalf("status = %q, want %q", snapshot.Status, service.HealthStatusDegraded)
	}
	for _, component := range []string{"postgres", "minio", "rabbitmq"} {
		if snapshot.Components[component] != service.HealthStatusDegraded {
			t.Fatalf("%s = %q, want %q", component, snapshot.Components[component], service.HealthStatusDegraded)
		}
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
