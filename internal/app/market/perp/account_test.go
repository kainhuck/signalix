package perp

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestPerpMarket_BalanceAndPosition(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{{
		Contract:   "ETH/USDT",
		Side:       perp.PositionLong,
		Size:       "2",
		EntryPrice: "100",
		UpdatedAt:  time.Unix(1, 0),
	}}
	ctx, cancel := context.WithCancel(context.Background())

	pm, err := NewPerpMarket(ctx, PerpMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		pm.Projection().Stop()
	}()
	if err := pm.Projection().Start(ctx); err != nil {
		t.Fatal(err)
	}

	bv, err := pm.Balance(ctx, "USDT")
	if err != nil {
		t.Fatal(err)
	}
	if bv.Available != "10000" {
		t.Fatalf("balance: %+v", bv)
	}

	pv, err := pm.Position(ctx, "ETH/USDT")
	if err != nil || pv == nil || pv.Size != "2" {
		t.Fatalf("position: %+v err=%v", pv, err)
	}
}

func TestPerpMarket_Ticker_notInCache(t *testing.T) {
	pm, err := NewPerpMarket(context.Background(), PerpMarketConfig{Exchange: testutil.NewStubExchange()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = pm.Ticker("BTC/USDT")
	if err == nil {
		t.Fatal("expected error")
	}
}
