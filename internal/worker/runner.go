package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
)

const (
	RoutingKeyUploaded        = "avatar.uploaded"
	RoutingKeyDeleteRequested = "avatar.delete_requested"
)

type Delivery struct {
	RoutingKey string
	Body       []byte
	Ack        func() error
	Nack       func(requeue bool) error
}

type Consumer interface {
	Consume(context.Context) (<-chan Delivery, error)
	Close() error
}

type Runner struct {
	consumer Consumer
	upload   *UploadHandler
	delete   *DeleteHandler
	log      *slog.Logger
}

func NewRunner(consumer Consumer, upload *UploadHandler, delete *DeleteHandler, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{consumer: consumer, upload: upload, delete: delete, log: log}
}

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
			r.log.Error("worker message failed", "routing_key", delivery.RoutingKey, "error", err.Error())
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
	switch delivery.RoutingKey {
	case RoutingKeyUploaded:
		id, err := parseAvatarID(delivery.Body)
		if err != nil {
			return err
		}
		return r.upload.Handle(ctx, UploadEvent{AvatarID: id})
	case RoutingKeyDeleteRequested:
		id, err := parseAvatarID(delivery.Body)
		if err != nil {
			return err
		}
		return r.delete.Handle(ctx, DeleteEvent{AvatarID: id})
	default:
		r.log.Warn("unknown worker routing key", "routing_key", delivery.RoutingKey)
		return nil
	}
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
