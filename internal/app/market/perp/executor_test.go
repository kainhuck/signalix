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
	return orderUserEventWithGateText(exchangeID, "", perp.OrderSubmitted)
}

func orderUserEventWithGateText(exchangeID, gateText string, status perp.OrderStatus) *perp.UserEvent {
	ev, err := perp.NewUserEvent(perp.UserOrderUpdate, &perp.OrderSnapshot{
		ExchangeOrderID: exchangeID,
		ClientID:        gateText,
		Status:          status,
	})
	if err != nil {
		panic(err)
	}
	return ev
}

func TestPerpExecutorUserEventClientID_UUID(t *testing.T) {
	t.Parallel()

	localID := "550e8400-e29b-41d4-a716-446655440000"
	gateText := perp.NormalizeClientOrderID(localID)

	ex := &pumpTestExchange{userCh: make(chan *perp.UserEvent, 4)}
	p := &PerpExecutor{
		exchange:        ex,
		orderEvents:     make(chan *models.OrderEvent, 4),
		gateTextToLocal: make(map[string]string),
	}
	p.registerGateText(localID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()

	ex.userCh <- orderUserEventWithGateText("ex-wrong", gateText, perp.OrderSubmitted)

	select {
	case oe := <-p.orderEvents:
		if oe.ClientID != localID {
			t.Fatalf("ClientID = %q, want %q", oe.ClientID, localID)
		}
		if oe.ExchangeID != "ex-wrong" {
			t.Fatalf("ExchangeID = %q", oe.ExchangeID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for order event")
	}
}

func TestPerpExecutorUserEventClientID_short(t *testing.T) {
	t.Parallel()

	localID := "abc123"
	gateText := perp.NormalizeClientOrderID(localID)

	ex := &pumpTestExchange{userCh: make(chan *perp.UserEvent, 4)}
	p := &PerpExecutor{
		exchange:        ex,
		orderEvents:     make(chan *models.OrderEvent, 4),
		gateTextToLocal: make(map[string]string),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()

	ex.userCh <- orderUserEventWithGateText("ex-1", gateText, perp.OrderSubmitted)

	select {
	case oe := <-p.orderEvents:
		if oe.ClientID != localID {
			t.Fatalf("ClientID = %q, want %q", oe.ClientID, localID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for order event")
	}
}

func TestPerpExecutorPlaceRegistersGateText(t *testing.T) {
	t.Parallel()

	localID := "550e8400-e29b-41d4-a716-446655440000"
	gateText := perp.NormalizeClientOrderID(localID)

	ex := &pumpTestExchange{userCh: make(chan *perp.UserEvent, 4)}
	p := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	ctx := context.Background()
	_, err := p.Place(ctx, &models.Order{
		ID:        localID,
		Symbol:    "BTC/USDT",
		Side:      models.OrderSideBuy,
		OrderType: models.OrderTypeMarket,
		Size:      "1",
	})
	if err != nil {
		t.Fatal(err)
	}

	p.gateTextMu.Lock()
	got, ok := p.gateTextToLocal[gateText]
	p.gateTextMu.Unlock()
	if !ok || got != localID {
		t.Fatalf("gate text index: ok=%v got=%q want=%q", ok, got, localID)
	}
}

func TestPerpExecutorPumpDropsWhenFull(t *testing.T) {
	t.Parallel()

	ex := &pumpTestExchange{userCh: make(chan *perp.UserEvent, 8)}
	p := &PerpExecutor{
		exchange:        ex,
		orderEvents:     make(chan *models.OrderEvent, 1),
		gateTextToLocal: make(map[string]string),
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
