package oms

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestExecutionEngine_DuplicateSubmitRejected(t *testing.T) {
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
	execs, pe := testPerpExecutors(ex)
	ee := NewExecutionEngine(execs, nil, WithContractMetaLookup(reg))
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		stopTestOMS(ee, pe)
	}()
	if err := startTestOMS(ctx, ee, pe); err != nil {
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

func TestExecutionEngine_applyOrderEvent_ClientIDFirst(t *testing.T) {
	t.Parallel()
	execs, _ := testPerpExecutors(nil)
	ee := NewExecutionEngine(execs, nil)
	ee.orders["local-1"] = &models.Order{
		ID:         "local-1",
		ExchangeID: "ex-other",
		Symbol:     "BTC/USDT",
		Status:     models.OrderStatusSubmitted,
		FilledSize: "0",
	}
	ee.applyOrderEvent(&models.OrderEvent{
		Market:     models.MarketPerp,
		ClientID:   "local-1",
		ExchangeID: "wrong-ex",
		Status:     models.OrderStatusFilled,
		FilledSize: "1",
		UpdatedAt:  time.Now(),
	})
	o, ok := ee.GetOrder("local-1")
	if !ok || o.Status != models.OrderStatusFilled || o.FilledSize != "1" {
		t.Fatalf("order after client id match: %+v ok=%v", o, ok)
	}
}
