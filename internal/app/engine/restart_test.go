package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestPruneCrashTimes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	window := 5 * time.Minute
	times := []time.Time{
		now.Add(-6 * time.Minute),
		now.Add(-4 * time.Minute),
		now.Add(-1 * time.Minute),
	}
	got := pruneCrashTimes(times, window, now)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestComputeBackoff(t *testing.T) {
	t.Parallel()
	cfg := config.RestartSettings{
		InitialBackoff:    3 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2,
	}
	if d := computeBackoff(cfg, 0); d != 3*time.Second {
		t.Fatalf("attempt 0: %v", d)
	}
	if d := computeBackoff(cfg, 1); d != 6*time.Second {
		t.Fatalf("attempt 1: %v", d)
	}
	if d := computeBackoff(cfg, 5); d != 60*time.Second {
		t.Fatalf("attempt 5 capped: %v", d)
	}
}

func TestCrashTrackerCircuit(t *testing.T) {
	t.Parallel()
	ct := newCrashTracker()
	now := time.Now()
	window := time.Minute
	name := "s1"

	for i := 0; i < 3; i++ {
		ct.record(name, now.Add(time.Duration(i)*time.Second))
	}
	count := ct.pruneAndCount(name, window, now.Add(2*time.Second))
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	e := &Engine{
		restartCfg: config.RestartSettings{
			MaxCrashes:  3,
			CrashWindow: window,
		},
		crashTracker: ct,
	}
	if !e.circuitOpen(name) {
		t.Fatal("expected circuit open")
	}
}

func TestHandleProcessExitNoDoubleHandle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	markets, err := NewTestPerpMarkets(testutil.NewStubExchange())
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{
		ctx:             ctx,
		markets:         markets,
		restartCfg:      config.RestartSettings{Enabled: false, CrashWindow: time.Minute, MaxCrashes: 3},
		crashTracker:    newCrashTracker(),
		strategyProcess: map[string]strategy.StrategyRuntime{},
	}

	sp := &recordingStrategyRuntime{name: "s1", ctx: ctx}
	e.strategyProcess["s1"] = sp

	e.handleProcessExit(sp)
	e.handleProcessExit(sp)

	if len(e.crashTracker.times["s1"]) != 1 {
		t.Fatalf("crash records = %d, want 1", len(e.crashTracker.times["s1"]))
	}
}
