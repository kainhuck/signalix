package oms

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestNonFinalOrderCountForStrategy(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	execs, pe := testPerpExecutors(ex)
	ee := NewExecutionEngine(execs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		stopTestOMS(ee, pe)
	}()
	if err := startTestOMS(ctx, ee, pe); err != nil {
		t.Fatal(err)
	}

	submit := func(id, strategy string) {
		t.Helper()
		o := &models.Order{
			ID:           id,
			Symbol:       "BTC/USDT",
			Side:         models.OrderSideBuy,
			OrderType:    models.OrderTypeLimit,
			Size:         "1",
			Status:       models.OrderStatusSubmitted,
			StrategyName: strategy,
		}
		if err := ee.SubmitOrder(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	submit("a1", "alpha")
	submit("a2", "alpha")
	submit("b1", "beta")

	if n := ee.NonFinalOrderCountForStrategy("alpha"); n != 2 {
		t.Fatalf("alpha = %d, want 2", n)
	}
	if n := ee.NonFinalOrderCountForStrategy("beta"); n != 1 {
		t.Fatalf("beta = %d, want 1", n)
	}
	if n := ee.NonFinalOrderCount(); n != 3 {
		t.Fatalf("total = %d, want 3", n)
	}
}
