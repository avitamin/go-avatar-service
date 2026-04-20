// Package rabbitmq provides a RabbitMQ-backed broker client and consumer.
package rabbitmq

import (
	"context"
	"errors"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

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
}

// Dial connects to RabbitMQ and declares the required topology.
func Dial(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	c := &Client{conn: conn, ch: ch, exchange: defaultExchange}
	if err := c.declareTopology(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// Publish sends a durable message to the configured exchange.
func (c *Client) Publish(ctx context.Context, topic string, msg []byte, messageID string) error {
	if c == nil || c.ch == nil {
		return errors.New("rabbitmq client is closed")
	}
	return c.ch.PublishWithContext(ctx, c.exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/octet-stream",
		DeliveryMode: amqp.Persistent,
		MessageId:    messageID,
		Type:         topic,
		Body:         msg,
	})
}

// HealthCheck verifies that the exchange is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return errors.New("rabbitmq client is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()
	return ch.ExchangeDeclarePassive(c.exchange, "topic", true, false, false, false, nil)
}

// Consume merges upload and delete queues into a single delivery stream.
func (c *Client) Consume(ctx context.Context) (<-chan worker.Delivery, error) {
	if c == nil || c.ch == nil {
		return nil, errors.New("rabbitmq client is closed")
	}
	if err := c.ch.Qos(1, 0, false); err != nil {
		return nil, err
	}
	uploads, err := c.ch.ConsumeWithContext(ctx, uploadQueue, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	deletes, err := c.ch.ConsumeWithContext(ctx, deleteQueue, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	out := make(chan worker.Delivery)
	var wg sync.WaitGroup
	merge := func(in <-chan amqp.Delivery) {
		defer wg.Done()
		for d := range in {
			delivery := d
			out <- worker.Delivery{
				RoutingKey: delivery.RoutingKey,
				Body:       delivery.Body,
				Ack:        func() error { return delivery.Ack(false) },
				Nack:       func(requeue bool) error { return delivery.Nack(false, requeue) },
			}
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
