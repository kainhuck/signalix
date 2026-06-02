package engine

import (
	"errors"
	"testing"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestBalanceSnapshot_projectionNotConfigured(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	_, err := e.BalanceSnapshot()
	if !errors.Is(err, ErrAccountProjectionNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceSnapshot_notReady(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	_, err := e.BalanceSnapshot()
	if !errors.Is(err, projection.ErrProjectionNotReady) {
		t.Fatalf("err = %v", err)
	}
}

func TestBalanceSnapshot_ok(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	bal, err := e.BalanceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if bal == nil || bal.Currency != "USDT" || bal.Available != "10000" {
		t.Fatalf("balance = %+v", bal)
	}
}

func TestPositionSnapshot_noPosition(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	pos, err := e.PositionSnapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if pos != nil {
		t.Fatalf("expected nil position, got %+v", pos)
	}
}

func TestAllPositionsSnapshot_withPositions(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{
			Contract: perp.Contract("BTC/USDT"),
			Side:     perp.PositionLong,
			Size:     "1",
		},
	}
	e := newTestEngine(t, t.TempDir(), ex)
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	list, err := e.AllPositionsSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Symbol != "BTC/USDT" {
		t.Fatalf("list = %+v", list)
	}
}

func TestAllPositionsSnapshot_empty(t *testing.T) {
	t.Parallel()
	e := newTestEngine(t, t.TempDir(), testutil.NewStubExchange())
	if err := e.accountProjection.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	list, err := e.AllPositionsSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}
