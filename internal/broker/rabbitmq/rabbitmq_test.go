package rabbitmq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/propagation"

	"go-avatar-service/internal/worker"
)

func TestTraceHeadersRoundTrip(t *testing.T) {
	ctx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})

	headers := injectTraceHeaders(ctx, nil)
	delivery := toWorkerDelivery(amqp.Delivery{
		RoutingKey: worker.RoutingKeyUploaded,
		Body:       []byte("avatar-1"),
		Headers:    headers,
	})

	if delivery.Headers["traceparent"] == "" {
		t.Fatalf("traceparent header missing: %#v", delivery.Headers)
	}
	if delivery.RoutingKey != worker.RoutingKeyUploaded || string(delivery.Body) != "avatar-1" {
		t.Fatalf("delivery mapping changed: %+v", delivery)
	}
}
