package perp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// pumpTestExchange 提供可写 UserEvents channel 的测试替身。
type pumpTestExchange struct {
	testutil.StubExchange
	userCh chan *perp.UserEvent
}

func (p *pumpTestExchange) UserEvents() <-chan *perp.UserEvent {
	return p.userCh
}

func orderUserEvent(exchangeID string) *perp.UserEvent {
	ev, err := perp.NewUserEvent(perp.UserOrderUpdate, &perp.OrderSnapshot{
		ExchangeOrderID: exchangeID,
		Status:          perp.OrderSubmitted,
	})
	if err != nil {
		panic(err)
	}
	return ev
}

func TestPerpExecutorPumpDropsWhenFull(t *testing.T) {
	t.Parallel()

	ex := &pumpTestExchange{userCh: make(chan *perp.UserEvent, 8)}
	p := &PerpExecutor{
		exchange:    ex,
		orderEvents: make(chan *models.OrderEvent, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()

	sent := make(chan struct{})
	go func() {
		for i := 0; i < 3; i++ {
			ex.userCh <- orderUserEvent(fmt.Sprintf("ex-%d", i))
		}
		close(sent)
	}()

	select {
	case <-sent:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pump blocked sending user events")
	}

	time.Sleep(20 * time.Millisecond)

	var received int
drain:
	for {
		select {
		case <-p.orderEvents:
			received++
		default:
			break drain
		}
	}
	if received != 1 {
		t.Fatalf("received %d order events, want 1 (rest dropped)", received)
	}
}

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
