package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	now := time.Unix(0, 0)
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenTimeout:      time.Second,
		Now:              func() time.Time { return now },
	})
	down := errors.New("dependency down")

	for i := 0; i < 2; i++ {
		err := breaker.Execute(context.Background(), func(context.Context) error {
			return down
		})
		if !errors.Is(err, down) {
			t.Fatalf("failure %d err = %v, want dependency error", i, err)
		}
	}

	err := breaker.Execute(context.Background(), func(context.Context) error {
		t.Fatal("open breaker must not call dependency")
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("open err = %v, want %v", err, ErrCircuitOpen)
	}

	now = now.Add(time.Second)
	err = breaker.Execute(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("half-open success err = %v", err)
	}

	err = breaker.Execute(context.Background(), func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("closed success err = %v", err)
	}
}

func TestCircuitBreakerDisabledPassesThrough(t *testing.T) {
	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          false,
		FailureThreshold: 1,
		OpenTimeout:      time.Minute,
	})
	down := errors.New("dependency down")

	for i := 0; i < 3; i++ {
		err := breaker.Execute(context.Background(), func(context.Context) error {
			return down
		})
		if !errors.Is(err, down) {
			t.Fatalf("err = %v, want dependency error", err)
		}
	}
}
