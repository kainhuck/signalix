package decision

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestDecisionEngine_ProcessSignal_LongNoPositionUsesStubExchange(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.ContractMetas = []*perp.ContractMeta{{
		Contract:         "BTC/USDT",
		QuantoMultiplier: "0.0001",
		OrderSizeMin:     "1",
	}}
	lookup := &stubTickerLookup{tickers: map[perp.Contract]*perp.TickerSnapshot{
		"BTC/USDT": {Contract: "BTC/USDT", MarkPrice: "50000"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	proj := projection.NewAccountProjection(ex)
	defer func() {
		cancel()
		proj.Stop()
	}()
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}
	de := NewDecisionEngine(proj, WithExchange(ex), WithTickerLookup(lookup))

	sig := &models.Signal{
		Symbol:    "BTC/USDT",
		Direction: models.DirectionLong,
		Strength:  0.5,
		Timestamp: 1,
	}

	order, err := de.ProcessSignal(ctx, "test-strat", sig)
	if err != nil {
		t.Fatalf("ProcessSignal: %v", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}
	if order.Side != models.OrderSideBuy {
		t.Fatalf("side: got %q want Buy", order.Side)
	}
	if order.OrderType != models.OrderTypeMarket {
		t.Fatalf("type: got %v want market", order.OrderType)
	}
	if order.Size != "200" {
		t.Fatalf("size: got %q want 200", order.Size)
	}
}
