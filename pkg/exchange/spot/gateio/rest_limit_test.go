package gateio

import (
	"context"
	"testing"
	"time"
)

func TestClient_waitREST_disabled(t *testing.T) {
	t.Parallel()
	c := NewClient("k", "s")
	if err := c.waitREST(context.Background()); err != nil {
		t.Fatalf("waitREST = %v", err)
	}
}

func TestClient_waitREST_withLimit(t *testing.T) {
	c := NewClient("k", "s", WithRateLimit(1))
	ctx := context.Background()
	if err := c.waitREST(ctx); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := c.waitREST(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 800*time.Millisecond {
		t.Fatalf("second waitREST too fast: %v", elapsed)
	}
}

func TestWithRateLimit_disabled(t *testing.T) {
	t.Parallel()
	c := NewClient("k", "s", WithRateLimit(0))
	if c.restLimiter == nil {
		t.Fatal("restLimiter should be non-nil when disabled")
	}
	if err := c.waitREST(context.Background()); err != nil {
		t.Fatalf("waitREST = %v", err)
	}
}
