package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go-avatar-service/internal/worker"
)

func BenchmarkRabbitMQPublishGetAckUpload(b *testing.B) {
	ctx := context.Background()
	client := benchmarkClient(b)
	body := []byte(`{"avatar_id":"bench-avatar"}`)
	messagePrefix := fmt.Sprintf("bench-%d", time.Now().UnixNano())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageID := fmt.Sprintf("%s-%d", messagePrefix, i)
		if err := client.Publish(ctx, worker.RoutingKeyUploaded, body, messageID); err != nil {
			b.Fatal(err)
		}
		delivery, ok, err := client.ch.Get(uploadQueue, false)
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			b.Fatal("published message was not available in upload queue")
		}
		if delivery.RoutingKey != worker.RoutingKeyUploaded {
			b.Fatalf("unexpected routing key %q", delivery.RoutingKey)
		}
		if err := delivery.Ack(false); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRabbitMQPublishGetAckDelete(b *testing.B) {
	ctx := context.Background()
	client := benchmarkClient(b)
	body := []byte(`{"avatar_id":"bench-avatar"}`)
	messagePrefix := fmt.Sprintf("bench-%d", time.Now().UnixNano())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageID := fmt.Sprintf("%s-%d", messagePrefix, i)
		if err := client.Publish(ctx, worker.RoutingKeyDeleteRequested, body, messageID); err != nil {
			b.Fatal(err)
		}
		delivery, ok, err := client.ch.Get(deleteQueue, false)
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			b.Fatal("published message was not available in delete queue")
		}
		if delivery.RoutingKey != worker.RoutingKeyDeleteRequested {
			b.Fatalf("unexpected routing key %q", delivery.RoutingKey)
		}
		if err := delivery.Ack(false); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkClient(b *testing.B) *Client {
	b.Helper()
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		b.Skip("RABBITMQ_URL is not set")
	}
	client, err := Dial(url)
	if err != nil {
		b.Fatal(err)
	}
	benchmarkPurgeQueues(b, client)
	b.Cleanup(func() {
		benchmarkPurgeQueues(b, client)
		_ = client.Close()
	})
	return client
}

func benchmarkPurgeQueues(b *testing.B, client *Client) {
	b.Helper()
	if _, err := client.ch.QueuePurge(uploadQueue, false); err != nil {
		b.Fatal(err)
	}
	if _, err := client.ch.QueuePurge(deleteQueue, false); err != nil {
		b.Fatal(err)
	}
}
