package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNew_disabled(t *testing.T) {
	t.Parallel()
	for _, rps := range []int{0, -1} {
		l := New(rps)
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("rps=%d: Wait = %v", rps, err)
		}
	}
	var nilLim *Limiter
	if err := nilLim.Wait(context.Background()); err != nil {
		t.Fatalf("nil Wait = %v", err)
	}
}

func TestWait_burst(t *testing.T) {
	l := New(2)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 400*time.Millisecond {
		t.Fatalf("third Wait too fast: %v", elapsed)
	}
}

func TestWait_contextCanceled(t *testing.T) {
	l := New(1)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(canceled); err != context.Canceled {
		t.Fatalf("Wait = %v, want context.Canceled", err)
	}
}
