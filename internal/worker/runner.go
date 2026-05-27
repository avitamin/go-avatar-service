// Package worker processes avatar events from the broker.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"go-avatar-service/internal/observability"
)

const (
	// RoutingKeyUploaded is emitted after the original avatar is stored.
	RoutingKeyUploaded = "avatar.uploaded"
	// RoutingKeyDeleteRequested is emitted after an avatar is soft-deleted.
	RoutingKeyDeleteRequested = "avatar.delete_requested"
)

// Delivery represents one broker message passed to the worker.
type Delivery struct {
	RoutingKey string
	Body       []byte
	Headers    map[string]any
	Ack        func() error
	Nack       func(requeue bool) error
}

// Consumer streams deliveries from the broker transport.
type Consumer interface {
	Consume(context.Context) (<-chan Delivery, error)
	Close() error
}

// Runner dispatches broker deliveries to upload and delete handlers.
type Runner struct {
	consumer Consumer
	upload   *UploadHandler
	delete   *DeleteHandler
	log      *slog.Logger
	metrics  *observability.Metrics
}

// Option customizes runner wiring.
type Option func(*Runner)

// WithObservability configures worker metrics and trace propagation.
func WithObservability(metrics *observability.Metrics) Option {
	return func(r *Runner) {
		r.metrics = metrics
	}
}

// NewRunner creates a Runner with the provided consumer and handlers.
func NewRunner(consumer Consumer, upload *UploadHandler, delete *DeleteHandler, log *slog.Logger, opts ...Option) *Runner {
	if log == nil {
		log = slog.Default()
	}
	r := &Runner{consumer: consumer, upload: upload, delete: delete, log: log}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run consumes deliveries until the consumer closes or the context is canceled.
func (r *Runner) Run(ctx context.Context) error {
	if r.consumer == nil {
		return errors.New("worker consumer is required")
	}
	deliveries, err := r.consumer.Consume(ctx)
	if err != nil {
		return err
	}
	defer r.consumer.Close()
	for delivery := range deliveries {
		if err := r.handle(ctx, delivery); err != nil {
			msgCtx := extractContext(ctx, delivery.Headers)
			r.log.ErrorContext(msgCtx, "worker message failed", observability.Attrs(msgCtx, "routing_key", delivery.RoutingKey, "error", err.Error())...)
			if delivery.Nack != nil {
				_ = delivery.Nack(true)
			}
			continue
		}
		if delivery.Ack != nil {
			_ = delivery.Ack()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil
	}
	return nil
}

func (r *Runner) handle(ctx context.Context, delivery Delivery) error {
	start := time.Now()
	ctx = extractContext(ctx, delivery.Headers)
	ctx, span := otel.Tracer("avatar-service/worker").Start(ctx, "worker handle "+delivery.RoutingKey)
	defer span.End()
	span.SetAttributes(attribute.String("messaging.rabbitmq.routing_key", delivery.RoutingKey))
	status := "success"
	defer func() {
		r.metrics.ObserveWorkerMessage(delivery.RoutingKey, status, time.Since(start))
	}()
	switch delivery.RoutingKey {
	case RoutingKeyUploaded:
		id, err := parseAvatarID(delivery.Body)
		if err != nil {
			status = "error"
			recordWorkerError(span, err)
			return err
		}
		err = r.upload.Handle(ctx, UploadEvent{AvatarID: id})
		if err != nil {
			status = "error"
			recordWorkerError(span, err)
			return err
		}
		return nil
	case RoutingKeyDeleteRequested:
		id, err := parseAvatarID(delivery.Body)
		if err != nil {
			status = "error"
			recordWorkerError(span, err)
			return err
		}
		err = r.delete.Handle(ctx, DeleteEvent{AvatarID: id})
		if err != nil {
			status = "error"
			recordWorkerError(span, err)
			return err
		}
		return nil
	default:
		status = "ignored"
		r.log.WarnContext(ctx, "unknown worker routing key", observability.Attrs(ctx, "routing_key", delivery.RoutingKey)...)
		return nil
	}
}

func extractContext(ctx context.Context, headers map[string]any) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	for key, value := range headers {
		if s, ok := value.(string); ok {
			carrier.Set(key, s)
		}
	}
	return propagation.TraceContext{}.Extract(ctx, carrier)
}

func recordWorkerError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func parseAvatarID(body []byte) (string, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", errors.New("avatar id is empty")
	}
	if body[0] == '{' {
		var event struct {
			AvatarID string `json:"avatar_id"`
		}
		if err := json.Unmarshal(body, &event); err != nil {
			return "", err
		}
		if event.AvatarID == "" {
			return "", errors.New("avatar id is empty")
		}
		return event.AvatarID, nil
	}
	var event struct {
		AvatarID string `json:"avatar_id"`
	}
	if err := json.Unmarshal(body, &event); err == nil && event.AvatarID != "" {
		return event.AvatarID, nil
	}
	return string(body), nil
}
