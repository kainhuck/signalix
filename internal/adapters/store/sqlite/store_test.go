package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

func TestStoreSaveOrderListNonTerminal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "signalix.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	p := "100"
	o := &models.Order{
		ID:           "oid-1",
		StrategyName: "s1",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeLimit,
		Size:         "1",
		FilledSize:   "0",
		Status:       models.OrderStatusSubmitted,
		Price:        &p,
		ExchangeID:   "ex-99",
		CreatedAt:    time.Unix(1000, 0).UTC(),
		UpdatedAt:    time.Unix(1001, 0).UTC(),
	}
	if err := s.SaveOrder(ctx, o); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 order, got %d", len(list))
	}
	if list[0].ID != o.ID || list[0].ExchangeID != o.ExchangeID || list[0].Status != models.OrderStatusSubmitted {
		t.Fatalf("unexpected row: %+v", list[0])
	}

	o.Status = models.OrderStatusFilled
	o.UpdatedAt = time.Unix(2000, 0).UTC()
	if err := s.SaveOrder(ctx, o); err != nil {
		t.Fatal(err)
	}
	list2, err := s.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list2) != 0 {
		t.Fatalf("terminal order should not list, got %d", len(list2))
	}
}

func TestStoreStrategyStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "st.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	raw := []byte(`{"x":1}`)
	ts := time.Now().UTC().Truncate(time.Second)
	if err := s.SaveStrategyState(ctx, "strat", "k1", raw, ts); err != nil {
		t.Fatal(err)
	}
	m, err := s.LoadAllStrategyStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(m["strat"]["k1"]) != `{"x":1}` {
		t.Fatalf("unexpected json: %q", string(m["strat"]["k1"]))
	}
}

func TestReconcileContractRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "c.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	o := &models.Order{
		ID:           "c1",
		StrategyName: "s",
		Symbol: "ETH/USDT",
		Side:         models.OrderSideSell,
		OrderType:    models.OrderTypeMarket,
		Size:         "2",
		FilledSize:   "0",
		Status:       models.OrderStatusPending,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.SaveOrder(ctx, o); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListNonTerminalOrders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Symbol != "ETH/USDT" {
		t.Fatalf("got %+v", list[0])
	}
}
