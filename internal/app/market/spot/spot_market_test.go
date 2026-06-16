package spot

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

type stubSpotExchange struct {
	pingCalled bool

	pairMeta []*spotex.PairMeta
	balances []*spotex.BalanceView
	klines   map[spotex.Pair][]*spotex.CandlestickSnapshot

	subs   []*spotex.Subscription
	unsubs []*spotex.Subscription

	publicEvents chan *spotex.PublicEvent
	userEvents   chan *spotex.UserEvent
}

var _ ports.SpotExchange = (*stubSpotExchange)(nil)

func newStubSpotExchange() *stubSpotExchange {
	return &stubSpotExchange{
		pairMeta: []*spotex.PairMeta{
			{Pair: spotex.Pair("BTC/USDT")},
			{Pair: spotex.Pair("ETH/USDT")},
		},
		balances: []*spotex.BalanceView{
			{Currency: "BTC", Total: "0.5", Available: "0.4", Frozen: "0.1", UpdatedAt: time.Unix(10, 0)},
			{Currency: "USDT", Total: "100", Available: "100", UpdatedAt: time.Unix(11, 0)},
		},
		klines: map[spotex.Pair][]*spotex.CandlestickSnapshot{
			spotex.Pair("BTC/USDT"): {
				{Pair: "BTC/USDT", Interval: "1m", Open: "1", High: "2", Low: "0.5", Close: "1.5", Volume: "10", VolumeBase: "10", TimestampSec: 1, WindowClosed: true},
				{Pair: "BTC/USDT", Interval: "1m", Open: "2", High: "3", Low: "1.5", Close: "2.5", Volume: "11", VolumeBase: "11", TimestampSec: 2, WindowClosed: true},
			},
		},
		publicEvents: make(chan *spotex.PublicEvent, 8),
		userEvents:   make(chan *spotex.UserEvent, 8),
	}
}

func (s *stubSpotExchange) Connect(context.Context, spotex.ConnectParts) error { return nil }
func (s *stubSpotExchange) Ping(context.Context) error {
	s.pingCalled = true
	return nil
}
func (s *stubSpotExchange) Close(context.Context) error { return nil }
func (s *stubSpotExchange) ListPairMeta(context.Context) ([]*spotex.PairMeta, error) {
	return s.pairMeta, nil
}
func (s *stubSpotExchange) ListCandlesticks(_ context.Context, q *spotex.ListCandlesticksQuery) ([]*spotex.CandlestickSnapshot, error) {
	return s.klines[q.Pair.Canonical()], nil
}
func (s *stubSpotExchange) Place(context.Context, *spotex.PlaceRequest) (*spotex.OrderSnapshot, error) {
	return nil, nil
}
func (s *stubSpotExchange) Cancel(context.Context, *spotex.CancelParams) error { return nil }
func (s *stubSpotExchange) GetOrder(context.Context, spotex.Pair, string) (*spotex.OrderSnapshot, error) {
	return nil, nil
}
func (s *stubSpotExchange) Balances(context.Context) ([]*spotex.BalanceView, error) {
	return s.balances, nil
}
func (s *stubSpotExchange) Balance(_ context.Context, currency string) (*spotex.BalanceView, error) {
	for _, bal := range s.balances {
		if bal.Currency == currency {
			return bal, nil
		}
	}
	return &spotex.BalanceView{Currency: currency}, nil
}
func (s *stubSpotExchange) Subscribe(_ context.Context, subs []*spotex.Subscription) error {
	s.subs = append(s.subs, subs...)
	return nil
}
func (s *stubSpotExchange) Unsubscribe(_ context.Context, subs []*spotex.Subscription) error {
	s.unsubs = append(s.unsubs, subs...)
	return nil
}
func (s *stubSpotExchange) PublicEvents() <-chan *spotex.PublicEvent { return s.publicEvents }
func (s *stubSpotExchange) UserEvents() <-chan *spotex.UserEvent     { return s.userEvents }

func TestSpotMarketBasicAccountAndTicker(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	var _ market.Market = sm

	if sm.Kind() != models.MarketSpot {
		t.Fatalf("Kind = %q, want spot", sm.Kind())
	}
	if err := sm.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !ex.pingCalled {
		t.Fatal("Ping did not call exchange")
	}
	if err := sm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := sm.Stop(); err != nil {
			t.Fatalf("Stop: %v", err)
		}
	}()

	bal, err := sm.Balance(ctx, "btc")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal.Currency != "BTC" || bal.Total != "0.5" || bal.Available != "0.4" {
		t.Fatalf("Balance = %+v", bal)
	}
	pos, err := sm.Position(ctx, "btcusdt")
	if err != nil {
		t.Fatalf("Position: %v", err)
	}
	if pos == nil || pos.Market != models.MarketSpot || pos.Symbol != "BTC/USDT" || pos.Size != "0.5" || pos.Side != "long" {
		t.Fatalf("Position = %+v", pos)
	}

	ev, err := spotex.NewPublicEvent(spotex.PublicTicker, &spotex.TickerSnapshot{
		Pair:            "BTC/USDT",
		Last:            "66000",
		ChangePct24h:    "1.2",
		Volume24hBase:   "12",
		Volume24hQuote:  "792000",
		TimestampMillis: 123,
	})
	if err != nil {
		t.Fatalf("NewPublicEvent: %v", err)
	}
	sm.router.OnPublicEvent(ev)
	ticker, err := sm.Ticker("BTC_USDT")
	if err != nil {
		t.Fatalf("Ticker: %v", err)
	}
	if ticker.Contract != "BTC/USDT" || ticker.Last != "66000" || ticker.MarkPrice != "66000" || ticker.Volume24h != "792000" {
		t.Fatalf("Ticker = %+v", ticker)
	}
}

func TestSpotMarketSubscribeWarmupAndUpdates(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex}, WithMarketBuffer(4))
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}

	req := market.SubscribeRequest{
		Strategy:   "s1",
		Symbols:    []string{"btcusdt"},
		Interval:   "1m",
		PushTicker: true,
	}
	if err := sm.Subscribe(ctx, req); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if len(ex.subs) != 2 {
		t.Fatalf("subscribe calls = %d, want 2", len(ex.subs))
	}
	if ex.subs[0].Channel != wsChannelSpotTickers || ex.subs[0].Pairs[0] != "BTC/USDT" {
		t.Fatalf("ticker sub = %+v", ex.subs[0])
	}
	if ex.subs[1].Channel != wsChannelSpotCandlesticks || ex.subs[1].Payload[0] != "1m" {
		t.Fatalf("candle sub = %+v", ex.subs[1])
	}

	payload, err := sm.WarmupHistory(ctx, req, 2)
	if err != nil {
		t.Fatalf("WarmupHistory: %v", err)
	}
	if payload == nil || len(payload.Series) != 1 || len(payload.Series[0].Bars) != 2 {
		t.Fatalf("HistoryPayload = %+v", payload)
	}
	klines, err := sm.Klines("BTC/USDT", "1m", 1)
	if err != nil {
		t.Fatalf("Klines: %v", err)
	}
	if len(klines) != 1 || klines[0].Close != "2.5" || !klines[0].WindowClosed {
		t.Fatalf("Klines = %+v", klines)
	}

	ev, err := spotex.NewPublicEvent(spotex.PublicCandlestick, &spotex.CandlestickSnapshot{
		Pair: "BTC/USDT", Interval: "1m", Open: "3", High: "4", Low: "2", Close: "3.5", Volume: "13", TimestampSec: 3, WindowClosed: true,
	})
	if err != nil {
		t.Fatalf("NewPublicEvent: %v", err)
	}
	sm.router.OnPublicEvent(ev)
	select {
	case upd := <-sm.Updates():
		if upd.Market != models.MarketSpot || upd.StrategyName != "s1" || upd.Kind != market.MarketUpdateKline || upd.Kline.Close != "3.5" {
			t.Fatalf("update = %+v", upd)
		}
	default:
		t.Fatal("expected kline update")
	}

	if err := sm.Unsubscribe("s1"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	if len(ex.unsubs) != 2 {
		t.Fatalf("unsubscribe calls = %d, want 2", len(ex.unsubs))
	}
}

func TestAccountProjectionBalanceUpdates(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	if err := sm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	ev, err := spotex.NewUserEvent(spotex.UserBalanceUpdate, &spotex.BalanceUpdateSnapshot{
		Currency: "ETH", Total: "2", Available: "1.5", Frozen: "0.5", UpdatedAt: time.Unix(20, 0),
	})
	if err != nil {
		t.Fatalf("NewUserEvent: %v", err)
	}
	sm.proj.OnUserEvent(ev)

	pos, err := sm.Position(ctx, "ETH/USDT")
	if err != nil {
		t.Fatalf("Position: %v", err)
	}
	if pos == nil || pos.Size != "2" || pos.Symbol != "ETH/USDT" {
		t.Fatalf("Position = %+v", pos)
	}
	positions, err := sm.ListPositions(ctx)
	if err != nil {
		t.Fatalf("ListPositions: %v", err)
	}
	if len(positions) != 2 {
		t.Fatalf("positions len = %d, want 2: %+v", len(positions), positions)
	}
}
