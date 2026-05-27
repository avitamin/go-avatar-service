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
