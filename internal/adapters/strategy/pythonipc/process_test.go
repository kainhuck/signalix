package pythonipc

import (
	"testing"
	"time"
)

func TestStrategyProcessCrashWindow(t *testing.T) {
	t.Parallel()
	sp := &StrategyProcess{
		crashes: []time.Time{
			time.Now().Add(-6 * time.Minute),
			time.Now().Add(-2 * time.Minute),
		},
	}
	window := 5 * time.Minute
	sp.PruneCrashes(window)
	if sp.GetCrashCount() != 1 {
		t.Fatalf("count = %d, want 1", sp.GetCrashCount())
	}
	sp.RecordCrash()
	if sp.GetCrashCount() != 2 {
		t.Fatalf("count after record = %d, want 2", sp.GetCrashCount())
	}
}
