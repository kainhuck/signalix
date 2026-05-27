package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter throttles REST requests in-process. When requestsPerSecond<=0, Wait is a no-op.
type Limiter struct {
	lim *rate.Limiter
}

// New creates a limiter. rps<=0 disables throttling.
func New(requestsPerSecond int) *Limiter {
	if requestsPerSecond <= 0 {
		return &Limiter{}
	}
	limit := rate.Limit(requestsPerSecond)
	return &Limiter{lim: rate.NewLimiter(limit, requestsPerSecond)}
}

// Wait blocks until a token is available or ctx is canceled.
func (l *Limiter) Wait(ctx context.Context) error {
	if l == nil || l.lim == nil {
		return nil
	}
	return l.lim.Wait(ctx)
}
