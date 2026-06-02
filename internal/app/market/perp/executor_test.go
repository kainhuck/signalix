package perp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	exchangeperp "github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
)

// pumpTestExchange 提供可写 UserEvents channel 的测试替身。
type pumpTestExchange struct {
	testutil.StubExchange
	userCh chan *exchangeperp.UserEvent
}

func (p *pumpTestExchange) UserEvents() <-chan *exchangeperp.UserEvent {
	return p.userCh
}

// gatePumpExchange 使用 Gate ClientOrderIDCodec，其余行为同 pumpTestExchange。
type gatePumpExchange struct {
	pumpTestExchange
	codec gateio.Client
}

func (g *gatePumpExchange) TagFromLocal(localID string) string {
	return g.codec.TagFromLocal(localID)
}

func (g *gatePumpExchange) LocalFromTag(tag string) (string, bool) {
	return g.codec.LocalFromTag(tag)
}

func orderUserEvent(exchangeID string) *exchangeperp.UserEvent {
	return orderUserEventWithClientTag(exchangeID, "", exchangeperp.OrderSubmitted)
}

func orderUserEventWithClientTag(exchangeID, clientTag string, status exchangeperp.OrderStatus) *exchangeperp.UserEvent {
	ev, err := exchangeperp.NewUserEvent(exchangeperp.UserOrderUpdate, &exchangeperp.OrderSnapshot{
		ExchangeOrderID: exchangeID,
		ClientID:        clientTag,
		Status:          status,
	})
	if err != nil {
		panic(err)
	}
	return ev
}

func TestNewPerpExecutor_requiresExchange(t *testing.T) {
	t.Parallel()
	_, err := NewPerpExecutor(PerpExecutorConfig{Exchange: nil})
	if err == nil {
		t.Fatal("expected error for nil exchange")
	}
}

func TestPerpExecutorUserEventClientID_UUID(t *testing.T) {
	t.Parallel()

	localID := "550e8400-e29b-41d4-a716-446655440000"
	ex := &gatePumpExchange{pumpTestExchange: pumpTestExchange{userCh: make(chan *exchangeperp.UserEvent, 4)}}
	clientTag := ex.TagFromLocal(localID)

	p := &PerpExecutor{
		exchange:    ex,
		codec:       ex,
		orderEvents: make(chan *models.OrderEvent, 4),
		tagToLocal:  make(map[string]string),
	}
	p.registerClientTag(localID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()

	ex.userCh <- orderUserEventWithClientTag("ex-wrong", clientTag, exchangeperp.OrderSubmitted)

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
	ex := &gatePumpExchange{pumpTestExchange: pumpTestExchange{userCh: make(chan *exchangeperp.UserEvent, 4)}}
	clientTag := ex.TagFromLocal(localID)

	p := &PerpExecutor{
		exchange:    ex,
		codec:       ex,
		orderEvents: make(chan *models.OrderEvent, 4),
		tagToLocal:  make(map[string]string),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()

	ex.userCh <- orderUserEventWithClientTag("ex-1", clientTag, exchangeperp.OrderSubmitted)

	select {
	case oe := <-p.orderEvents:
		if oe.ClientID != localID {
			t.Fatalf("ClientID = %q, want %q", oe.ClientID, localID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for order event")
	}
}

func TestPerpExecutorPlaceRegistersClientTag(t *testing.T) {
	t.Parallel()

	localID := "550e8400-e29b-41d4-a716-446655440000"
	ex := &gatePumpExchange{pumpTestExchange: pumpTestExchange{userCh: make(chan *exchangeperp.UserEvent, 4)}}
	clientTag := ex.TagFromLocal(localID)

	p, err := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = p.Place(ctx, &models.Order{
		ID:        localID,
		Symbol:    "BTC/USDT",
		Side:      models.OrderSideBuy,
		OrderType: models.OrderTypeMarket,
		Size:      "1",
	})
	if err != nil {
		t.Fatal(err)
	}

	p.tagMu.Lock()
	got, ok := p.tagToLocal[clientTag]
	p.tagMu.Unlock()
	if !ok || got != localID {
		t.Fatalf("client tag index: ok=%v got=%q want=%q", ok, got, localID)
	}
}

func TestPerpExecutorPumpDropsWhenFull(t *testing.T) {
	t.Parallel()

	ex := &pumpTestExchange{userCh: make(chan *exchangeperp.UserEvent, 8)}
	p, err := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	p.orderEvents = make(chan *models.OrderEvent, 1)

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
	p, err := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
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
	ex.GetOrderHook = func(_ context.Context, _ exchangeperp.Contract, _ string) (*exchangeperp.OrderSnapshot, error) {
		return &exchangeperp.OrderSnapshot{
			ExchangeOrderID: "ex-99",
			Status:          exchangeperp.OrderFilled,
			FilledSize:      "1",
		}, nil
	}
	p, err := NewPerpExecutor(PerpExecutorConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
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
