package projection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestAccountProjection_Snapshot_NotReady(t *testing.T) {
	ex := testutil.NewStubExchange()
	p := NewAccountProjection(ex)
	_, _, _, err := p.Snapshot("BTC/USDT")
	if !errors.Is(err, ErrProjectionNotReady) {
		t.Fatalf("Snapshot: got %v want ErrProjectionNotReady", err)
	}
}

func TestAccountProjection_Snapshot_RevisionAndPositions(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{
			Contract:   "ETH/USDT",
			Side:       perp.PositionLong,
			Size:       "2.5",
			EntryPrice: "100",
			MarkPrice:  "101",
			UpdatedAt:  time.Unix(1, 0),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	proj := NewAccountProjection(ex)
	defer func() {
		cancel()
		proj.Stop()
	}()
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_, posBTC, rev1, err := proj.Snapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if posBTC != nil {
		t.Fatalf("BTC position: got %+v want nil", posBTC)
	}
	if rev1 != 1 {
		t.Fatalf("revision after first refresh: got %d want 1", rev1)
	}

	bal, posETH, rev2, err := proj.Snapshot("ETH/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if bal == nil || bal.Available != "10000" {
		t.Fatalf("balance: got %+v", bal)
	}
	if posETH == nil || posETH.Size != "2.5" || posETH.Contract != "ETH/USDT" {
		t.Fatalf("ETH position: got %+v", posETH)
	}
	if rev2 != rev1 {
		t.Fatalf("revision mismatch: %d vs %d", rev2, rev1)
	}
}

func TestAccountProjection_OpenPositionCount(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "A/USDT", Side: perp.PositionLong, Size: "1"},
		{Contract: "B/USDT", Side: perp.PositionShort, Size: "2"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	proj := NewAccountProjection(ex)
	defer func() {
		cancel()
		proj.Stop()
	}()
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := proj.OpenPositionCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("OpenPositionCount: got %d want 2", n)
	}
}

func TestAccountProjection_OnUserEvent_notReady_discarded(t *testing.T) {
	ex := testutil.NewStubExchange()
	p := NewAccountProjection(ex)
	ev, err := perp.NewUserEvent(perp.UserPositionUpdate, &perp.PositionSnapshot{
		Contract: "BTC/USDT",
		Side:     perp.PositionLong,
		Size:     "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(context.Background(), ev)
	_, _, rev, err := p.Snapshot("BTC/USDT")
	if !errors.Is(err, ErrProjectionNotReady) {
		t.Fatalf("Snapshot: got %v", err)
	}
	if rev != 0 {
		t.Fatalf("revision: got %d want 0", rev)
	}
}

func TestAccountProjection_OnUserEvent_position_upsert(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := testutil.NewStubExchange()
	p := NewAccountProjection(ex)
	defer func() {
		cancel()
		p.Stop()
	}()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ev, err := perp.NewUserEvent(perp.UserPositionUpdate, &perp.PositionSnapshot{
		Contract:   "BTC/USDT",
		Side:       perp.PositionLong,
		Size:       "3",
		EntryPrice: "50000",
		UpdatedAt:  time.Unix(2, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(ctx, ev)

	_, pos, rev, err := p.Snapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if rev != 2 {
		t.Fatalf("revision: got %d want 2", rev)
	}
	if pos == nil || pos.Size != "3" {
		t.Fatalf("position: got %+v", pos)
	}
}

func TestAccountProjection_OnUserEvent_position_flat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "ETH/USDT", Side: perp.PositionLong, Size: "2"},
	}
	p := NewAccountProjection(ex)
	defer func() {
		cancel()
		p.Stop()
	}()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ev, err := perp.NewUserEvent(perp.UserPositionUpdate, &perp.PositionSnapshot{
		Contract: "ETH/USDT",
		Size:     "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(ctx, ev)

	_, pos, _, err := p.Snapshot("ETH/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if pos != nil {
		t.Fatalf("position: got %+v want nil", pos)
	}
	n, err := p.OpenPositionCount()
	if err != nil || n != 0 {
		t.Fatalf("OpenPositionCount: n=%d err=%v", n, err)
	}
}

func TestAccountProjection_OnUserEvent_balance_total(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := testutil.NewStubExchange()
	ex.BalanceAvailable = "10000"
	p := NewAccountProjection(ex)
	defer func() {
		cancel()
		p.Stop()
	}()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ev, err := perp.NewUserEvent(perp.UserBalanceUpdate, &perp.BalanceUpdateSnapshot{
		Currency:  "USDT",
		Balance:   "9000",
		UpdatedAt: time.Unix(3, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(ctx, ev)

	bal, _, rev, err := p.Snapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if rev != 2 {
		t.Fatalf("revision: got %d want 2", rev)
	}
	if bal.Total != "9000" || bal.Available != "10000" {
		t.Fatalf("balance: got %+v", bal)
	}
}

func TestAccountProjection_OnUserEvent_order_noop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := NewAccountProjection(testutil.NewStubExchange())
	defer func() {
		cancel()
		p.Stop()
	}()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_, _, revBefore, err := p.Snapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := perp.NewUserEvent(perp.UserOrderUpdate, &perp.OrderSnapshot{
		ExchangeOrderID: "ex-1",
		Contract:        "BTC/USDT",
		Side:            perp.SideBuy,
		Type:            perp.OrderTypeMarket,
		Size:            "1",
		Status:          perp.OrderSubmitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(ctx, ev)
	_, _, revAfter, err := p.Snapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if revAfter != revBefore {
		t.Fatalf("revision: before %d after %d", revBefore, revAfter)
	}
}

func TestAccountProjection_refresh_overwrites_ws(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "ETH/USDT", Side: perp.PositionLong, Size: "2.5"},
	}
	p := NewAccountProjection(ex)
	defer func() {
		cancel()
		p.Stop()
	}()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ev, err := perp.NewUserEvent(perp.UserPositionUpdate, &perp.PositionSnapshot{
		Contract: "ETH/USDT",
		Side:     perp.PositionLong,
		Size:     "99",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.OnUserEvent(ctx, ev)

	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "ETH/USDT", Side: perp.PositionLong, Size: "1"},
	}
	if err := p.refresh(ctx); err != nil {
		t.Fatal(err)
	}

	_, pos, _, err := p.Snapshot("ETH/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if pos == nil || pos.Size != "1" {
		t.Fatalf("position after REST refresh: got %+v", pos)
	}
}
