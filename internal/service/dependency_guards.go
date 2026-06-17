package service

import (
	"context"
	"time"

	"go-avatar-service/internal/domain"
)

// GuardedRepository applies a circuit breaker around repository calls.
type GuardedRepository struct {
	next    Repository
	breaker *CircuitBreaker
}

// NewGuardedRepository wraps a Repository with dependency protection.
func NewGuardedRepository(next Repository, breaker *CircuitBreaker) Repository {
	if next == nil || breaker == nil {
		return next
	}
	return &GuardedRepository{next: next, breaker: breaker}
}

func (r *GuardedRepository) Create(ctx context.Context, avatar *domain.Avatar) error {
	return r.breaker.Execute(ctx, func(ctx context.Context) error {
		return r.next.Create(ctx, avatar)
	})
}

func (r *GuardedRepository) GetActiveByID(ctx context.Context, id string) (out *domain.Avatar, err error) {
	err = r.breaker.Execute(ctx, func(ctx context.Context) error {
		out, err = r.next.GetActiveByID(ctx, id)
		return err
	})
	return out, err
}

func (r *GuardedRepository) GetByID(ctx context.Context, id string) (out *domain.Avatar, err error) {
	err = r.breaker.Execute(ctx, func(ctx context.Context) error {
		out, err = r.next.GetByID(ctx, id)
		return err
	})
	return out, err
}

func (r *GuardedRepository) ListActiveByUser(ctx context.Context, userID string) (out []domain.Avatar, err error) {
	err = r.breaker.Execute(ctx, func(ctx context.Context) error {
		out, err = r.next.ListActiveByUser(ctx, userID)
		return err
	})
	return out, err
}

func (r *GuardedRepository) SoftDeleteByID(ctx context.Context, id string, deletedAt time.Time) error {
	return r.breaker.Execute(ctx, func(ctx context.Context) error {
		return r.next.SoftDeleteByID(ctx, id, deletedAt)
	})
}

func (r *GuardedRepository) MarkPublishFailed(ctx context.Context, id string) error {
	return r.breaker.Execute(ctx, func(ctx context.Context) error {
		return r.next.MarkPublishFailed(ctx, id)
	})
}

func (r *GuardedRepository) UpdateProcessingResult(ctx context.Context, id, thumb100, thumb300 string) error {
	return r.breaker.Execute(ctx, func(ctx context.Context) error {
		return r.next.UpdateProcessingResult(ctx, id, thumb100, thumb300)
	})
}

// GuardedStorage applies a circuit breaker around object storage calls.
type GuardedStorage struct {
	next    Storage
	breaker *CircuitBreaker
}

// NewGuardedStorage wraps a Storage with dependency protection.
func NewGuardedStorage(next Storage, breaker *CircuitBreaker) Storage {
	if next == nil || breaker == nil {
		return next
	}
	return &GuardedStorage{next: next, breaker: breaker}
}

func (s *GuardedStorage) Put(ctx context.Context, key string, data []byte, mime string) error {
	return s.breaker.Execute(ctx, func(ctx context.Context) error {
		return s.next.Put(ctx, key, data, mime)
	})
}

func (s *GuardedStorage) Get(ctx context.Context, key string) (data []byte, mime string, err error) {
	err = s.breaker.Execute(ctx, func(ctx context.Context) error {
		data, mime, err = s.next.Get(ctx, key)
		return err
	})
	return data, mime, err
}

func (s *GuardedStorage) Delete(ctx context.Context, key string) error {
	return s.breaker.Execute(ctx, func(ctx context.Context) error {
		return s.next.Delete(ctx, key)
	})
}

func (s *GuardedStorage) Exists(ctx context.Context, key string) (exists bool, err error) {
	err = s.breaker.Execute(ctx, func(ctx context.Context) error {
		exists, err = s.next.Exists(ctx, key)
		return err
	})
	return exists, err
}

// GuardedBroker applies a circuit breaker around broker publication.
type GuardedBroker struct {
	next    Broker
	breaker *CircuitBreaker
}

// NewGuardedBroker wraps a Broker with dependency protection.
func NewGuardedBroker(next Broker, breaker *CircuitBreaker) Broker {
	if next == nil || breaker == nil {
		return next
	}
	return &GuardedBroker{next: next, breaker: breaker}
}

func (b *GuardedBroker) Publish(ctx context.Context, topic string, msg []byte, messageID string) error {
	return b.breaker.Execute(ctx, func(ctx context.Context) error {
		return b.next.Publish(ctx, topic, msg, messageID)
	})
}
