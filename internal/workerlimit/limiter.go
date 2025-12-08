package workerlimit

import "context"

// Limiter provides a simple global semaphore to cap concurrent work.
// If max <= 0 the limiter is disabled.
type Limiter struct {
	sem     chan struct{}
	enabled bool
}

// New creates a limiter with the given maximum parallelism.
// max <= 0 disables limiting.
func New(max int) *Limiter {
	if max <= 0 {
		return &Limiter{enabled: false}
	}
	return &Limiter{
		sem:     make(chan struct{}, max),
		enabled: true,
	}
}

// Acquire blocks until a slot is available or context is cancelled.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l == nil || !l.enabled {
		return nil
	}
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot. Safe to call only after successful Acquire.
func (l *Limiter) Release() {
	if l == nil || !l.enabled {
		return
	}
	select {
	case <-l.sem:
	default:
	}
}
