package perp

import (
	"context"
	"sync"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

type recordingExchange struct {
	mu        sync.Mutex
	subscribe []*perp.Subscription
	pubCh     chan *perp.PublicEvent
}

func newRecordingExchange() *recordingExchange {
	return &recordingExchange{
		pubCh: make(chan *perp.PublicEvent, 16),
	}
}

func (r *recordingExchange) Ping(context.Context) error { return nil }
func (r *recordingExchange) Close(context.Context) error {
	return nil
}
func (r *recordingExchange) Connect(context.Context, perp.ConnectParts) error { return nil }
func (r *recordingExchange) ListContractMeta(context.Context) ([]*perp.ContractMeta, error) {
	return nil, nil
}
func (r *recordingExchange) ListCandlesticks(context.Context, *perp.ListCandlesticksQuery) ([]*perp.CandlestickSnapshot, error) {
	return nil, nil
}
func (r *recordingExchange) Subscribe(_ context.Context, subs []*perp.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribe = append(r.subscribe, subs...)
	return nil
}
func (r *recordingExchange) Unsubscribe(context.Context, []*perp.Subscription) error { return nil }
func (r *recordingExchange) PublicEvents() <-chan *perp.PublicEvent                  { return r.pubCh }
func (r *recordingExchange) Place(context.Context, *perp.PlaceRequest) (*perp.OrderSnapshot, error) {
	return nil, nil
}
func (r *recordingExchange) Cancel(context.Context, *perp.CancelParams) error { return nil }
func (r *recordingExchange) GetOrder(context.Context, perp.Contract, string) (*perp.OrderSnapshot, error) {
	return nil, nil
}
func (r *recordingExchange) Positions(context.Context) ([]*perp.PositionSnapshot, error) {
	return nil, nil
}
func (r *recordingExchange) Position(context.Context, perp.Contract) (*perp.PositionSnapshot, error) {
	return nil, nil
}
func (r *recordingExchange) Balance(context.Context) (*perp.BalanceView, error) {
	return nil, nil
}
func (r *recordingExchange) UserEvents() <-chan *perp.UserEvent {
	ch := make(chan *perp.UserEvent)
	return ch
}

func (r *recordingExchange) tickerSubscribeCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.subscribe {
		if s.Channel == wsChannelTickers {
			n++
		}
	}
	return n
}

func (r *recordingExchange) candlestickSubscribeCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.subscribe {
		if s.Channel == wsChannelCandlesticks {
			n++
		}
	}
	return n
}

func TestSubscribeWithoutTicker(t *testing.T) {
	t.Parallel()

	ex := newRecordingExchange()
	mr := NewMarketRouter(ex)
	if err := mr.SubscribeContracts("s1", []perp.Contract{"BTC/USDT"}, "5m", false); err != nil {
		t.Fatal(err)
	}
	if got := ex.tickerSubscribeCount(); got != 1 {
		t.Fatalf("ticker subscribe calls = %d, want 1 (engine cache)", got)
	}
	if got := ex.candlestickSubscribeCount(); got != 1 {
		t.Fatalf("candlestick subscribe calls = %d, want 1", got)
	}

	tick, _ := perp.NewPublicEvent(perp.PublicTicker, &perp.TickerSnapshot{
		Contract: "BTC/USDT",
		Last:     "1",
	})
	mr.OnPublicEvent(tick)
	select {
	case <-mr.Updates():
		t.Fatal("expected no tick fan-out when subscribe_ticker=false")
	default:
	}
	if got, ok := mr.GetCachedTicker("BTC/USDT"); !ok || got.Last != "1" {
		t.Fatalf("ticker cache: ok=%v %+v", ok, got)
	}
}

func TestSubscribeWithTickerPushesToStrategy(t *testing.T) {
	t.Parallel()

	ex := newRecordingExchange()
	mr := NewMarketRouter(ex, WithBufferSize(4))
	if err := mr.SubscribeContracts("s1", []perp.Contract{"BTC/USDT"}, "5m", true); err != nil {
		t.Fatal(err)
	}
	tick, _ := perp.NewPublicEvent(perp.PublicTicker, &perp.TickerSnapshot{
		Contract: "BTC/USDT",
		Last:     "2",
	})
	mr.OnPublicEvent(tick)
	select {
	case u := <-mr.Updates():
		if u.Kind != market.MarketUpdateTicker || u.StrategyName != "s1" {
			t.Fatalf("unexpected update: %+v", u)
		}
	default:
		t.Fatal("expected tick fan-out when subscribe_ticker=true")
	}
}

func TestSubscribeCandlesticksDedup(t *testing.T) {
	t.Parallel()

	ex := newRecordingExchange()
	mr := NewMarketRouter(ex)

	if err := mr.SubscribeContracts("s1", []perp.Contract{"BTC/USDT"}, "5m", true); err != nil {
		t.Fatal(err)
	}
	if err := mr.SubscribeContracts("s2", []perp.Contract{"BTC/USDT"}, "5m", true); err != nil {
		t.Fatal(err)
	}
	if got := ex.candlestickSubscribeCount(); got != 1 {
		t.Fatalf("candlestick subscribe calls = %d, want 1", got)
	}
}

func TestOnCandlestickClosedOnlyFanout(t *testing.T) {
	t.Parallel()

	ex := newRecordingExchange()
	mr := NewMarketRouter(ex, WithBufferSize(8))
	if err := mr.SubscribeContracts("s1", []perp.Contract{"BTC/USDT"}, "1m", true); err != nil {
		t.Fatal(err)
	}

	open, _ := perp.NewPublicEvent(perp.PublicCandlestick, &perp.CandlestickSnapshot{
		Contract:     "BTC/USDT",
		Interval:     "1m",
		Close:        "1",
		WindowClosed: false,
	})
	mr.OnPublicEvent(open)

	closed, _ := perp.NewPublicEvent(perp.PublicCandlestick, &perp.CandlestickSnapshot{
		Contract:     "BTC/USDT",
		Interval:     "1m",
		Close:        "2",
		WindowClosed: true,
	})
	mr.OnPublicEvent(closed)

	var updates []market.MarketUpdate
drain:
	for {
		select {
		case u := <-mr.Updates():
			updates = append(updates, u)
		default:
			break drain
		}
	}

	if len(updates) != 1 {
		t.Fatalf("got %d updates, want 1 closed kline", len(updates))
	}
	if updates[0].Kind != market.MarketUpdateKline {
		t.Fatalf("kind = %v, want kline", updates[0].Kind)
	}
	if updates[0].StrategyName != "s1" || updates[0].Kline.Close != "2" {
		t.Fatalf("unexpected update: %+v", updates[0])
	}
}

func TestUnsubscribeAllClearsCandleSubs(t *testing.T) {
	t.Parallel()

	ex := newRecordingExchange()
	mr := NewMarketRouter(ex)
	if err := mr.SubscribeContracts("s1", []perp.Contract{"ETH/USDT"}, "5m", true); err != nil {
		t.Fatal(err)
	}
	if err := mr.UnsubscribeAll("s1"); err != nil {
		t.Fatal(err)
	}

	closed, _ := perp.NewPublicEvent(perp.PublicCandlestick, &perp.CandlestickSnapshot{
		Contract:     "ETH/USDT",
		Interval:     "5m",
		WindowClosed: true,
	})
	mr.OnPublicEvent(closed)

	select {
	case u := <-mr.Updates():
		t.Fatalf("unexpected update after unsubscribe: %+v", u)
	default:
	}
}
