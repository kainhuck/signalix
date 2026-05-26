package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

func TestNotifyStrategyLogSubscribers_filters(t *testing.T) {
	e := &Engine{ctx: context.Background()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chAll := e.RegisterStrategyLogSubscriber(ctx, "", 0, 4)
	chAlpha := e.RegisterStrategyLogSubscriber(ctx, "alpha", 0, 4)
	chMin := e.RegisterStrategyLogSubscriber(ctx, "", 2, 4)

	e.notifyStrategyLogSubscribers(&models.StrategyLogRow{ID: 1, StrategyName: "alpha", Message: "a"})
	e.notifyStrategyLogSubscribers(&models.StrategyLogRow{ID: 2, StrategyName: "beta", Message: "b"})
	e.notifyStrategyLogSubscribers(&models.StrategyLogRow{ID: 3, StrategyName: "alpha", Message: "c"})

	assertReceivedIDs(t, chAll, []int64{1, 2, 3})
	assertReceivedIDs(t, chAlpha, []int64{1, 3})
	assertReceivedIDs(t, chMin, []int64{3})
}

func TestNotifyStrategyLogSubscribers_skipsZeroID(t *testing.T) {
	e := &Engine{ctx: context.Background()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := e.RegisterStrategyLogSubscriber(ctx, "", 0, 1)
	e.notifyStrategyLogSubscribers(&models.StrategyLogRow{ID: 0, StrategyName: "s", Message: "x"})
	select {
	case <-ch:
		t.Fatal("unexpected row for zero ID")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRegisterStrategyLogSubscriber_ctxCancelCloses(t *testing.T) {
	e := &Engine{ctx: context.Background()}
	ctx, cancel := context.WithCancel(context.Background())
	ch := e.RegisterStrategyLogSubscriber(ctx, "", 0, 1)
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func assertReceivedIDs(t *testing.T, ch <-chan *models.StrategyLogRow, want []int64) {
	t.Helper()
	got := make([]int64, 0, len(want))
	deadline := time.After(500 * time.Millisecond)
	for len(got) < len(want) {
		select {
		case row := <-ch:
			if row == nil {
				t.Fatal("nil row")
			}
			got = append(got, row.ID)
		case <-deadline:
			t.Fatalf("timeout: got %v want %v", got, want)
		}
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
