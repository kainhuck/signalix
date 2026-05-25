package oms

import (
	"context"
	"strings"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
)

func TestExecutionEngine_DuplicateSubmitRejected(t *testing.T) {
	ex := testutil.NewStubExchange()
	ee := NewExecutionEngine(ex, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = ee.Stop()
	}()

	if err := ee.Start(ctx); err != nil {
		t.Fatal(err)
	}

	o := &models.Order{
		ID:           "dup-test-id",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeMarket,
		Size:         "1",
		FilledSize:   "0",
		StrategyName: "s",
	}
	if err := ee.SubmitOrder(ctx, o); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	o2 := &models.Order{
		ID:           "dup-test-id",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideSell,
		OrderType:    models.OrderTypeMarket,
		Size:         "2",
		FilledSize:   "0",
		StrategyName: "s",
	}
	if err := ee.SubmitOrder(ctx, o2); err == nil {
		t.Fatal("expected duplicate order id error")
	} else if !strings.Contains(err.Error(), "duplicate order id") {
		t.Fatalf("unexpected err: %v", err)
	}
}
