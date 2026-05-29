package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig controls per-client request limiting.
type RateLimitConfig struct {
	Enabled               bool
	RequestsPerSec        float64
	Burst                 int
	TrustForwardedHeaders bool
	Now                   func() time.Time
	BucketTTL             time.Duration
	CleanupInterval       time.Duration
}

// RateLimiter is a small per-client token bucket limiter.
type RateLimiter struct {
	rate  float64
	burst float64
	now   func() time.Time

	trustForwardedHeaders bool
	bucketTTL             time.Duration
	cleanupInterval       time.Duration
	lastCleanup           time.Time

	mu      sync.Mutex
	buckets map[string]*rateBucket
}

type rateBucket struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

// NewRateLimiter creates a per-client limiter. Invalid numeric values fall back to one request burst.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	rate := cfg.RequestsPerSec
	if rate <= 0 {
		rate = 1
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = 1
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	bucketTTL := cfg.BucketTTL
	if bucketTTL <= 0 {
		bucketTTL = 5 * time.Minute
	}
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}
	return &RateLimiter{
		rate:                  rate,
		burst:                 float64(burst),
		now:                   now,
		trustForwardedHeaders: cfg.TrustForwardedHeaders,
		bucketTTL:             bucketTTL,
		cleanupInterval:       cleanupInterval,
		lastCleanup:           now(),
		buckets:               make(map[string]*rateBucket),
	}
}

// Middleware rejects requests that exceed the configured per-client bucket.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l != nil && !l.allow(clientKey(r, l.trustForwardedHeaders)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.cleanupInterval {
		l.cleanup(now)
	}

	bucket, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &rateBucket{tokens: l.burst - 1, last: now, lastSeen: now}
		return true
	}

	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * l.rate
		if bucket.tokens > l.burst {
			bucket.tokens = l.burst
		}
		bucket.last = now
	}
	bucket.lastSeen = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func (l *RateLimiter) cleanup(now time.Time) {
	for key, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) >= l.bucketTTL {
			delete(l.buckets, key)
		}
	}
	l.lastCleanup = now
}

func clientKey(r *http.Request, trustForwardedHeaders bool) string {
	if trustForwardedHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}
