package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/adapters/store/sqlite"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestReconcileWithExchange_OrderNotFoundMarksCancelled(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "r.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	o := &models.Order{
		ID:           "local-1",
		StrategyName: "st",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeLimit,
		Size:         "1",
		FilledSize:   "0",
		Status:       models.OrderStatusSubmitted,
		ExchangeID:   "missing-on-exchange",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := st.SaveOrder(ctx, o); err != nil {
		t.Fatal(err)
	}

	orders, err := st.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ex := testutil.NewStubExchange()
	reconcileWithExchange(ctx, ex, st, orders)

	again, err := st.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no non-terminal after reconcile not-found, got %+v", again)
	}
}

func TestReconcileWithExchange_AppliesExchangeView(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "r2.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	o := &models.Order{
		ID:           "local-2",
		StrategyName: "st",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeLimit,
		Size:         "1",
		FilledSize:   "0",
		Status:       models.OrderStatusSubmitted,
		ExchangeID:   "ex-ok",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := st.SaveOrder(ctx, o); err != nil {
		t.Fatal(err)
	}
	orders, err := st.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}

	ex := testutil.NewStubExchange()
	ex.GetOrderHook = func(ctx context.Context, contract perp.Contract, orderID string) (*perp.OrderSnapshot, error) {
		if orderID != "ex-ok" {
			return nil, perp.NewError(perp.ErrOrderNotFound, "no", nil)
		}
		return &perp.OrderSnapshot{
			ExchangeOrderID: "ex-ok",
			Contract:        contract,
			Status:          perp.OrderPartialFilled,
			FilledSize:      "0.5",
			UpdatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}, nil
	}
	reconcileWithExchange(ctx, ex, st, orders)

	orders2, err := st.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders2) != 1 {
		t.Fatalf("want 1 open order, got %d", len(orders2))
	}
	if orders2[0].Status != models.OrderStatusPartialFilled || orders2[0].FilledSize != "0.5" {
		t.Fatalf("unexpected updated order: %+v", orders2[0])
	}
}
