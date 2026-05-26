package projection

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestAccountProjection_RefreshHook_onSuccess(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "BTC/USDT", Side: perp.PositionLong, Size: "1"},
	}

	var calls atomic.Int32
	proj := NewAccountProjection(ex, WithRefreshHook(func(balance *perp.BalanceView, positions []*perp.PositionSnapshot, revision uint64, at time.Time) {
		calls.Add(1)
		if balance == nil || balance.Total == "" {
			t.Errorf("balance: %+v", balance)
		}
		if revision == 0 || at.IsZero() {
			t.Errorf("revision=%d at=%v", revision, at)
		}
		if len(positions) != 1 {
			t.Errorf("positions: %+v", positions)
		}
	}))

	ctx := context.Background()
	if err := proj.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("hook calls: %d", calls.Load())
	}
}

func TestAccountProjection_RefreshHook_onFailure(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.BalanceErr = errors.New("balance unavailable")

	var calls atomic.Int32
	proj := NewAccountProjection(ex, WithRefreshHook(func(*perp.BalanceView, []*perp.PositionSnapshot, uint64, time.Time) {
		calls.Add(1)
	}))

	if err := proj.refresh(context.Background()); err == nil {
		t.Fatal("expected refresh error")
	}
	if calls.Load() != 0 {
		t.Fatalf("hook calls: %d want 0", calls.Load())
	}
}

func TestAccountProjection_RefreshHook_notOnUserEvent(t *testing.T) {
	ctx := context.Background()
	ex := testutil.NewStubExchange()
	var calls atomic.Int32
	proj := NewAccountProjection(ex, WithRefreshHook(func(*perp.BalanceView, []*perp.PositionSnapshot, uint64, time.Time) {
		calls.Add(1)
	}))

	if err := proj.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	base := calls.Load()
	if base != 1 {
		t.Fatalf("baseline hook calls: %d want 1", base)
	}

	ev, err := perp.NewUserEvent(perp.UserPositionUpdate, &perp.PositionSnapshot{
		Contract: "BTC/USDT",
		Side:     perp.PositionLong,
		Size:     "3",
	})
	if err != nil {
		t.Fatal(err)
	}
	proj.OnUserEvent(ctx, ev)

	if calls.Load() != base {
		t.Fatalf("hook calls: %d want baseline %d", calls.Load(), base)
	}
}

func TestAccountProjection_SetRefreshHook_nil(t *testing.T) {
	proj := NewAccountProjection(testutil.NewStubExchange())
	proj.SetRefreshHook(nil)
	if err := proj.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
}
