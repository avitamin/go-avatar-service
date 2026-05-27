package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen reports that a dependency circuit breaker is currently open.
var ErrCircuitOpen = errors.New("dependency circuit breaker is open")

// CircuitBreakerConfig controls dependency circuit breaker behavior.
type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold int
	OpenTimeout      time.Duration
	Now              func() time.Time
	IsFailure        func(error) bool
}

// CircuitBreaker is a small closed/open/half-open dependency guard.
type CircuitBreaker struct {
	enabled          bool
	failureThreshold int
	openTimeout      time.Duration
	now              func() time.Time
	isFailure        func(error) bool

	mu           sync.Mutex
	failures     int
	openUntil    time.Time
	halfOpenCall bool
}

// NewCircuitBreaker creates a circuit breaker with defensive defaults.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	threshold := cfg.FailureThreshold
	if threshold <= 0 {
		threshold = 5
	}
	timeout := cfg.OpenTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CircuitBreaker{
		enabled:          cfg.Enabled,
		failureThreshold: threshold,
		openTimeout:      timeout,
		now:              now,
		isFailure:        cfg.IsFailure,
	}
}

// Execute runs an operation unless the circuit is open.
func (b *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	if b == nil || !b.enabled {
		return operation(ctx)
	}
	if err := b.beforeCall(); err != nil {
		return err
	}
	err := operation(ctx)
	b.afterCall(err)
	return err
}

func (b *CircuitBreaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	if b.openUntil.IsZero() {
		return nil
	}
	if now.Before(b.openUntil) {
		return ErrCircuitOpen
	}
	if b.halfOpenCall {
		return ErrCircuitOpen
	}
	b.halfOpenCall = true
	return nil
}

func (b *CircuitBreaker) afterCall(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err == nil || (b.isFailure != nil && !b.isFailure(err)) {
		b.failures = 0
		b.openUntil = time.Time{}
		b.halfOpenCall = false
		return
	}
	b.halfOpenCall = false
	b.failures++
	if b.failures >= b.failureThreshold {
		b.openUntil = b.now().Add(b.openTimeout)
	}
}
