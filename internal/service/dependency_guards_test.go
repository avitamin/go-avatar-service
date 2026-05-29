package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGuardedStorageExistsReturnsDependencyError(t *testing.T) {
	ctx := context.Background()
	storageErr := errors.New("storage down")
	next := &stubStorage{existsErr: storageErr}
	guarded := NewGuardedStorage(next, NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenTimeout:      time.Minute,
	}))

	exists, err := guarded.Exists(ctx, "avatar.jpg")
	if exists {
		t.Fatal("Exists() exists = true, want false")
	}
	if !errors.Is(err, storageErr) {
		t.Fatalf("Exists() err = %v, want %v", err, storageErr)
	}
}

func TestGuardedStorageExistsOpensCircuitOnRepeatedDependencyErrors(t *testing.T) {
	ctx := context.Background()
	storageErr := errors.New("storage down")
	next := &stubStorage{existsErr: storageErr}
	guarded := NewGuardedStorage(next, NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 2,
		OpenTimeout:      time.Minute,
	}))

	for i := 0; i < 2; i++ {
		if _, err := guarded.Exists(ctx, "avatar.jpg"); !errors.Is(err, storageErr) {
			t.Fatalf("Exists() err = %v, want %v", err, storageErr)
		}
	}

	exists, err := guarded.Exists(ctx, "avatar.jpg")
	if exists {
		t.Fatal("Exists() exists = true, want false")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Exists() err = %v, want %v", err, ErrCircuitOpen)
	}
	if next.existsCalls != 2 {
		t.Fatalf("next Exists calls = %d, want 2", next.existsCalls)
	}
}

func TestGuardedStorageExistsAbsentObjectDoesNotOpenCircuit(t *testing.T) {
	ctx := context.Background()
	next := &stubStorage{exists: false}
	guarded := NewGuardedStorage(next, NewCircuitBreaker(CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 1,
		OpenTimeout:      time.Minute,
	}))

	exists, err := guarded.Exists(ctx, "missing.jpg")
	if err != nil {
		t.Fatalf("Exists() err = %v, want nil", err)
	}
	if exists {
		t.Fatal("Exists() exists = true, want false")
	}
	if err := guarded.Put(ctx, "avatar.jpg", []byte("data"), "image/jpeg"); err != nil {
		t.Fatalf("Put() err = %v, want nil after absent Exists", err)
	}
}

type stubStorage struct {
	exists      bool
	existsErr   error
	existsCalls int
}

func (s *stubStorage) Put(context.Context, string, []byte, string) error {
	return nil
}

func (s *stubStorage) Get(context.Context, string) ([]byte, string, error) {
	return nil, "", ErrObjectNotFound
}

func (s *stubStorage) Delete(context.Context, string) error {
	return nil
}

func (s *stubStorage) Exists(context.Context, string) (bool, error) {
	s.existsCalls++
	return s.exists, s.existsErr
}
