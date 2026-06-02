package market

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestPerpExecutorPlace(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	p := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	ctx := context.Background()
	o := &models.Order{
		ID:        "ord-1",
		Symbol:    "BTC/USDT",
		Side:      models.OrderSideBuy,
		OrderType: models.OrderTypeMarket,
		Size:      "1",
	}
	exID, err := p.Place(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if exID != "stub-ex-1" {
		t.Fatalf("exchange id = %q", exID)
	}
}

func TestPerpExecutorSync(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ex.GetOrderHook = func(_ context.Context, _ perp.Contract, _ string) (*perp.OrderSnapshot, error) {
		return &perp.OrderSnapshot{
			ExchangeOrderID: "ex-99",
			Status:          perp.OrderFilled,
			FilledSize:      "1",
		}, nil
	}
	p := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	oe, err := p.Sync(context.Background(), &models.Order{
		ID:         "local-1",
		Symbol:     "BTC/USDT",
		ExchangeID: "ex-99",
	})
	if err != nil {
		t.Fatal(err)
	}
	if oe.ClientID != "local-1" || oe.ExchangeID != "ex-99" || oe.Status != models.OrderStatusFilled {
		t.Fatalf("unexpected event: %+v", oe)
	}
}
