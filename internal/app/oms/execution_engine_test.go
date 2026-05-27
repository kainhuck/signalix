package oms

import (
	"context"
	"strings"
	"testing"

	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
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

func TestExecutionEngine_SubmitRejectedInvalidSize(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.ContractMetas = []*perp.ContractMeta{{
		Contract:      "BTC/USDT",
		OrderSizeMin:  "10",
		OrderSizeStep: "1",
	}}
	reg := instrument.NewRegistry()
	if err := reg.LoadFrom(context.Background(), ex); err != nil {
		t.Fatal(err)
	}
	ee := NewExecutionEngine(ex, nil, nil, WithContractMetaLookup(reg))
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = ee.Stop()
	}()
	if err := ee.Start(ctx); err != nil {
		t.Fatal(err)
	}

	o := &models.Order{
		ID:           "invalid-size-id",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeMarket,
		Size:         "1",
		FilledSize:   "0",
		StrategyName: "s",
	}
	if err := ee.SubmitOrder(ctx, o); err == nil {
		t.Fatal("expected validation error")
	}
	if o.Status != models.OrderStatusRejected {
		t.Fatalf("status = %s want Rejected", o.Status)
	}
	ex.Mu.Lock()
	calls := ex.PlaceCalls
	ex.Mu.Unlock()
	if calls != 0 {
		t.Fatalf("PlaceCalls = %d want 0", calls)
	}
}
