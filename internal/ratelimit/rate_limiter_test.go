package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	limiter := NewLimiter(10, 5) // 10 req/sec, burst 5

	// Test that initial requests are allowed
	for i := 0; i < 5; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// Test that burst is exhausted
	if limiter.Allow("test-key") {
		t.Error("Request after burst should be rate limited")
	}

	// Wait for token refill
	time.Sleep(150 * time.Millisecond)

	// Should allow one more request
	if !limiter.Allow("test-key") {
		t.Error("Request after refill should be allowed")
	}
}

func TestRateLimiterHTTPMiddleware(t *testing.T) {
	limiter := NewLimiter(100, 20)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := limiter.HTTPMiddleware(handler)

	// Test allowed request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test rate limited request (exhaust burst)
	for i := 0; i < 25; i++ {
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}

	// Next request should be rate limited
	w = httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	limiter := NewLimiter(10, 5)

	// Different keys should have separate buckets
	if !limiter.Allow("key1") {
		t.Error("Key1 should be allowed")
	}

	if !limiter.Allow("key2") {
		t.Error("Key2 should be allowed")
	}

	// Exhaust key1
	for i := 0; i < 5; i++ {
		limiter.Allow("key1")
	}

	// key1 should be limited
	if limiter.Allow("key1") {
		t.Error("Key1 should be rate limited")
	}

	// key2 should still work
	if !limiter.Allow("key2") {
		t.Error("Key2 should still be allowed")
	}
}

