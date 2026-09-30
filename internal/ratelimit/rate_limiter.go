package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/metrics"
	"github.com/rs/zerolog/log"
)

// Limiter implements rate limiting
type Limiter struct {
	requestsPerSecond int
	burst             int
	mu                sync.RWMutex
	tokens            map[string]*tokenBucket
	cleanupInterval   time.Duration
	lastCleanup       time.Time
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewLimiter creates a new rate limiter
func NewLimiter(requestsPerSecond int, burst int) *Limiter {
	limiter := &Limiter{
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
		tokens:            make(map[string]*tokenBucket),
		cleanupInterval:  5 * time.Minute,
		lastCleanup:       time.Now(),
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request should be allowed
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get or create token bucket for this key
	bucket, exists := l.tokens[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(l.burst),
			lastUpdate: time.Now(),
		}
		l.tokens[key] = bucket
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	tokensToAdd := elapsed * float64(l.requestsPerSecond)
	bucket.tokens = min(bucket.tokens+tokensToAdd, float64(l.burst))
	bucket.lastUpdate = now

	// Check if we have enough tokens
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// cleanup removes old token buckets periodically
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, bucket := range l.tokens {
			bucket.mu.Lock()
			// Remove buckets that haven't been used in 10 minutes
			if now.Sub(bucket.lastUpdate) > 10*time.Minute {
				delete(l.tokens, key)
			}
			bucket.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// HTTPMiddleware returns an HTTP middleware for rate limiting
func (l *Limiter) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use IP address as key
		key := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			key = forwarded
		}

		if !l.Allow(key) {
			log.Warn().
				Str("ip", key).
				Str("path", r.URL.Path).
				Msg("Rate limit exceeded")

			// Record metric
			metrics.RecordRateLimitRejected()

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

