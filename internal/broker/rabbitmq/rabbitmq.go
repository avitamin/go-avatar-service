// Package rabbitmq provides a RabbitMQ-backed broker client and consumer.
package rabbitmq

import (
	"context"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"go-avatar-service/internal/observability"
	"go-avatar-service/internal/worker"
)

const (
	defaultExchange = "avatars"
	uploadQueue     = "avatars.uploads"
	deleteQueue     = "avatars.deletes"
)

// Client publishes and consumes avatar worker events through RabbitMQ.
type Client struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
	once     sync.Once
	metrics  *observability.Metrics
}

// Option customizes client wiring.
type Option func(*Client)

// WithObservability configures RabbitMQ metrics and traces.
func WithObservability(metrics *observability.Metrics) Option {
	return func(c *Client) {
		c.metrics = metrics
	}
}

// Dial connects to RabbitMQ and declares the required topology.
func Dial(url string, opts ...Option) (*Client, error) {
	start := time.Now()
	_, span := otel.Tracer("avatar-service/rabbitmq").Start(context.Background(), "rabbitmq Dial")
	conn, err := amqp.Dial(url)
	if err != nil {
		recordRabbitError(span, err)
		span.End()
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		recordRabbitError(span, err)
		span.End()
		_ = conn.Close()
		return nil, err
	}
	c := &Client{conn: conn, ch: ch, exchange: defaultExchange}
	for _, opt := range opts {
		opt(c)
	}
	if err := c.declareTopology(); err != nil {
		recordRabbitError(span, err)
		c.metrics.ObserveDependencyOperation("rabbitmq", "Dial", "error", time.Since(start))
		span.End()
		_ = c.Close()
		return nil, err
	}
	c.metrics.ObserveDependencyOperation("rabbitmq", "Dial", "success", time.Since(start))
	span.End()
	return c, nil
}

// Publish sends a durable message to the configured exchange.
func (c *Client) Publish(ctx context.Context, topic string, msg []byte, messageID string) error {
	ctx, span, done := c.start(ctx, "Publish")
	defer done(nil)
	span.SetAttributes(attribute.String("messaging.destination", c.exchange), attribute.String("messaging.rabbitmq.routing_key", topic), attribute.String("messaging.message.id", messageID))
	if c == nil || c.ch == nil {
		err := errors.New("rabbitmq client is closed")
		recordRabbitError(span, err)
		done(err)
		return err
	}
	err := c.ch.PublishWithContext(ctx, c.exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/octet-stream",
		DeliveryMode: amqp.Persistent,
		Headers:      injectTraceHeaders(ctx, nil),
		MessageId:    messageID,
		Type:         topic,
		Body:         msg,
	})
	if err != nil {
		recordRabbitError(span, err)
		done(err)
	}
	return err
}

// HealthCheck verifies that the exchange is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, span, done := c.start(ctx, "HealthCheck")
	defer done(nil)
	if c == nil || c.conn == nil {
		err := errors.New("rabbitmq client is closed")
		recordRabbitError(span, err)
		done(err)
		return err
	}
	if err := ctx.Err(); err != nil {
		recordRabbitError(span, err)
		done(err)
		return err
	}
	ch, err := c.conn.Channel()
	if err != nil {
		recordRabbitError(span, err)
		done(err)
		return err
	}
	defer func() { _ = ch.Close() }()
	err = ch.ExchangeDeclarePassive(c.exchange, "topic", true, false, false, false, nil)
	if err != nil {
		recordRabbitError(span, err)
		done(err)
	}
	return err
}

// Consume merges upload and delete queues into a single delivery stream.
func (c *Client) Consume(ctx context.Context) (<-chan worker.Delivery, error) {
	ctx, span, done := c.start(ctx, "Consume")
	defer done(nil)
	if c == nil || c.ch == nil {
		err := errors.New("rabbitmq client is closed")
		recordRabbitError(span, err)
		done(err)
		return nil, err
	}
	if err := c.ch.Qos(1, 0, false); err != nil {
		recordRabbitError(span, err)
		done(err)
		return nil, err
	}
	uploads, err := c.ch.ConsumeWithContext(ctx, uploadQueue, "", false, false, false, false, nil)
	if err != nil {
		recordRabbitError(span, err)
		done(err)
		return nil, err
	}
	deletes, err := c.ch.ConsumeWithContext(ctx, deleteQueue, "", false, false, false, false, nil)
	if err != nil {
		recordRabbitError(span, err)
		done(err)
		return nil, err
	}
	out := make(chan worker.Delivery)
	var wg sync.WaitGroup
	merge := func(in <-chan amqp.Delivery) {
		defer wg.Done()
		for d := range in {
			delivery := d
			out <- toWorkerDelivery(delivery)
		}
	}
	wg.Add(2)
	go merge(uploads)
	go merge(deletes)
	go func() {
		wg.Wait()
		close(out)
	}()
	return out, nil
}

func (c *Client) start(ctx context.Context, operation string) (context.Context, trace.Span, func(error)) {
	start := time.Now()
	ctx, span := otel.Tracer("avatar-service/rabbitmq").Start(ctx, "rabbitmq "+operation)
	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", defaultExchange),
	)
	called := false
	return ctx, span, func(err error) {
		if called {
			return
		}
		called = true
		status := "success"
		if err != nil {
			status = "error"
		}
		if c != nil {
			c.metrics.ObserveDependencyOperation("rabbitmq", operation, status, time.Since(start))
		}
		span.End()
	}
}

func recordRabbitError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func injectTraceHeaders(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	for key, value := range carrier {
		headers[key] = value
	}
	return headers
}

func toWorkerDelivery(delivery amqp.Delivery) worker.Delivery {
	headers := make(map[string]any, len(delivery.Headers))
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	return worker.Delivery{
		RoutingKey: delivery.RoutingKey,
		Body:       delivery.Body,
		Headers:    headers,
		Ack:        func() error { return delivery.Ack(false) },
		Nack:       func(requeue bool) error { return delivery.Nack(false, requeue) },
	}
}

// Close releases the AMQP channel and connection once.
func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		if c.ch != nil {
			err = c.ch.Close()
		}
		if c.conn != nil {
			if connErr := c.conn.Close(); err == nil {
				err = connErr
			}
		}
	})
	return err
}

func (c *Client) declareTopology() error {
	if err := c.ch.ExchangeDeclare(c.exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := c.declareQueue(uploadQueue, worker.RoutingKeyUploaded); err != nil {
		return err
	}
	return c.declareQueue(deleteQueue, worker.RoutingKeyDeleteRequested)
}

func (c *Client) declareQueue(name, routingKey string) error {
	if _, err := c.ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
		return err
	}
	return c.ch.QueueBind(name, routingKey, c.exchange, false, nil)
}
