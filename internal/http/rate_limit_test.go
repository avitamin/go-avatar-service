package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-avatar-service/internal/service"
)

func TestRateLimiterRejectsWithJSONError(t *testing.T) {
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(
		svc,
		staticHealthService{snapshot: service.HealthSnapshot{Status: service.HealthStatusOK}},
		WithRateLimiter(RateLimitConfig{
			Enabled:        true,
			RequestsPerSec: 1,
			Burst:          1,
			Now:            func() time.Time { return time.Unix(0, 0) },
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.0.2.10:12345"

	first := httptest.NewRecorder()
	router.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	assertErrorModel(t, second.Result())
}

func TestRateLimiterCanBeDisabled(t *testing.T) {
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(
		svc,
		staticHealthService{snapshot: service.HealthSnapshot{Status: service.HealthStatusOK}},
		WithRateLimiter(RateLimitConfig{
			Enabled:        false,
			RequestsPerSec: 1,
			Burst:          1,
			Now:            func() time.Time { return time.Unix(0, 0) },
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	for i := 0; i < 3; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status[%d] = %d, want %d", i, resp.Code, http.StatusOK)
		}
	}
}

func TestRateLimiterRemovesStaleBucketsAfterTTL(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSec:  1,
		Burst:           1,
		Now:             func() time.Time { return now },
		BucketTTL:       time.Minute,
		CleanupInterval: time.Second,
	})

	if !limiter.allow("stale-client") {
		t.Fatal("first request was denied")
	}
	if len(limiter.buckets) != 1 {
		t.Fatalf("buckets after first request = %d, want 1", len(limiter.buckets))
	}

	now = now.Add(time.Minute)
	if !limiter.allow("fresh-client") {
		t.Fatal("request from fresh client was denied")
	}

	if _, ok := limiter.buckets["stale-client"]; ok {
		t.Fatal("stale bucket was not removed")
	}
	if _, ok := limiter.buckets["fresh-client"]; !ok {
		t.Fatal("fresh bucket was not created")
	}
}

func TestRateLimiterKeepsRecentlyActiveBucketDuringCleanup(t *testing.T) {
	now := time.Unix(200, 0)
	limiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSec:  1,
		Burst:           1,
		Now:             func() time.Time { return now },
		BucketTTL:       time.Minute,
		CleanupInterval: 30 * time.Second,
	})

	if !limiter.allow("stale-client") {
		t.Fatal("first stale-client request was denied")
	}

	now = now.Add(45 * time.Second)
	if !limiter.allow("active-client") {
		t.Fatal("first active-client request was denied")
	}

	now = now.Add(30 * time.Second)
	if !limiter.allow("trigger-client") {
		t.Fatal("trigger-client request was denied")
	}

	if _, ok := limiter.buckets["stale-client"]; ok {
		t.Fatal("stale bucket was not removed")
	}
	if _, ok := limiter.buckets["active-client"]; !ok {
		t.Fatal("recently active bucket was removed")
	}
}

func TestRateLimiterCleanupDoesNotResetActiveExhaustedBucketBeforeTTL(t *testing.T) {
	now := time.Unix(300, 0)
	limiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSec:  0.000001,
		Burst:           1,
		Now:             func() time.Time { return now },
		BucketTTL:       time.Minute,
		CleanupInterval: 10 * time.Second,
	})

	if !limiter.allow("active-client") {
		t.Fatal("first active-client request was denied")
	}
	if limiter.allow("active-client") {
		t.Fatal("second active-client request was allowed")
	}

	now = now.Add(10 * time.Second)
	if !limiter.allow("trigger-client") {
		t.Fatal("trigger-client request was denied")
	}
	if limiter.allow("active-client") {
		t.Fatal("active-client bucket was reset before TTL")
	}
}
