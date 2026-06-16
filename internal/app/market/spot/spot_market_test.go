package spot

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/shopspring/decimal"
)

type stubSpotExchange struct {
	pingCalled bool

	pairMeta []*spotex.PairMeta
	balances []*spotex.BalanceView
	klines   map[spotex.Pair][]*spotex.CandlestickSnapshot

	subs   []*spotex.Subscription
	unsubs []*spotex.Subscription

	placeReq   *spotex.PlaceRequest
	cancelReq  *spotex.CancelParams
	getOrder   *spotex.OrderSnapshot
	getOrderID string

	publicEvents chan *spotex.PublicEvent
	userEvents   chan *spotex.UserEvent
}

var _ ports.SpotExchange = (*stubSpotExchange)(nil)

func newStubSpotExchange() *stubSpotExchange {
	return &stubSpotExchange{
		pairMeta: []*spotex.PairMeta{
			{Pair: spotex.Pair("BTC/USDT"), MinBaseAmount: "0.0001", MinQuoteAmount: "10", MaxBaseAmount: "10", AmountPrecision: 4, TradeStatus: "tradable"},
			{Pair: spotex.Pair("ETH/USDT"), MinBaseAmount: "0.001", MinQuoteAmount: "10", MaxBaseAmount: "100", AmountPrecision: 3, TradeStatus: "tradable"},
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
func (s *stubSpotExchange) Place(_ context.Context, req *spotex.PlaceRequest) (*spotex.OrderSnapshot, error) {
	s.placeReq = req
	return &spotex.OrderSnapshot{OrderID: "ex-1", ExchangeOrderID: "ex-1", ClientID: req.ClientID, Pair: req.Pair, Status: spotex.OrderSubmitted, UpdatedAt: time.Unix(30, 0)}, nil
}
func (s *stubSpotExchange) Cancel(_ context.Context, req *spotex.CancelParams) error {
	s.cancelReq = req
	return nil
}
func (s *stubSpotExchange) GetOrder(_ context.Context, pair spotex.Pair, orderID string) (*spotex.OrderSnapshot, error) {
	s.getOrderID = orderID
	if s.getOrder != nil {
		return s.getOrder, nil
	}
	return &spotex.OrderSnapshot{OrderID: orderID, ExchangeOrderID: orderID, Pair: pair, Status: spotex.OrderFilled, FilledSize: "1", UpdatedAt: time.Unix(31, 0)}, nil
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

func TestSpotDeciderMarketBuyUsesQuoteAmount(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	ex.balances = []*spotex.BalanceView{
		{Currency: "USDT", Total: "1000", Available: "1000", UpdatedAt: time.Unix(1, 0)},
	}
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex, DecisionSizeDivisor: 10})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	if err := sm.proj.Start(ctx); err != nil {
		t.Fatalf("projection start: %v", err)
	}

	order, err := sm.Decide(ctx, "s1", &models.Signal{
		Symbol:    "BTC/USDT",
		Direction: models.DirectionLong,
		Strength:  1,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if order == nil || order.Market != models.MarketSpot || order.Side != models.OrderSideBuy || order.OrderType != models.OrderTypeMarket || order.Size != "100" {
		t.Fatalf("order = %+v", order)
	}
}

func TestSpotDeciderLimitBuyFixedSizingConvertsToBase(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	ex.balances = []*spotex.BalanceView{
		{Currency: "USDT", Total: "1000", Available: "1000", UpdatedAt: time.Unix(1, 0)},
	}
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	if err := sm.proj.Start(ctx); err != nil {
		t.Fatalf("projection start: %v", err)
	}

	mode := models.SizingModeFixed
	value := "250"
	price := "50000"
	order, err := sm.Decide(ctx, "s1", &models.Signal{
		Symbol:     "BTC/USDT",
		Direction:  models.DirectionLong,
		Strength:   1,
		Price:      &price,
		SizingMode: &mode,
		Value:      &value,
		Timestamp:  time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if order == nil || order.Side != models.OrderSideBuy || order.OrderType != models.OrderTypeLimit || order.Size != "0.005" {
		t.Fatalf("order = %+v", order)
	}
}

func TestSpotDeciderShortAndFlatSellHoldings(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	if err := sm.proj.Start(ctx); err != nil {
		t.Fatalf("projection start: %v", err)
	}

	for _, dir := range []models.Direction{models.DirectionShort, models.DirectionFlat} {
		order, err := sm.Decide(ctx, "s1", &models.Signal{
			Symbol:    "BTC/USDT",
			Direction: dir,
			Strength:  1,
			Timestamp: time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("Decide %s: %v", dir, err)
		}
		if order == nil || order.Side != models.OrderSideSell || order.Size != "0.4" {
			t.Fatalf("order for %s = %+v", dir, order)
		}
	}
}

func TestSpotExecutorPlaceCancelSyncAndUserEvents(t *testing.T) {
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

	order := &models.Order{
		ID:         "local-1",
		Market:     models.MarketSpot,
		Symbol:     "BTC/USDT",
		Side:       models.OrderSideBuy,
		OrderType:  models.OrderTypeMarket,
		Size:       "100",
		ExchangeID: "ex-1",
	}
	exID, err := sm.Place(ctx, order)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if exID != "ex-1" || ex.placeReq == nil || ex.placeReq.QuoteAmount == nil || *ex.placeReq.QuoteAmount != "100" || ex.placeReq.Size != "" {
		t.Fatalf("place req = %+v, exID=%s", ex.placeReq, exID)
	}
	if err := sm.Cancel(ctx, order); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ex.cancelReq == nil || ex.cancelReq.OrderID != "ex-1" || ex.cancelReq.ClientOrderID != "local-1" {
		t.Fatalf("cancel req = %+v", ex.cancelReq)
	}
	ev, err := sm.Sync(ctx, order)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if ev.Market != models.MarketSpot || ev.ExchangeID != "ex-1" || ev.ClientID != "local-1" || ev.Status != models.OrderStatusFilled {
		t.Fatalf("sync event = %+v", ev)
	}

	balEv, err := spotex.NewUserEvent(spotex.UserBalanceUpdate, &spotex.BalanceUpdateSnapshot{
		Currency: "ETH", Total: "3", Available: "2.5", UpdatedAt: time.Unix(40, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	orderEv, err := spotex.NewUserEvent(spotex.UserOrderUpdate, &spotex.OrderSnapshot{
		OrderID: "ex-2", ExchangeOrderID: "ex-2", ClientID: "local-2", Pair: "ETH/USDT", Status: spotex.OrderFilled, FilledSize: "1.2", UpdatedAt: time.Unix(41, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	ex.userEvents <- balEv
	ex.userEvents <- orderEv

	select {
	case got := <-sm.OrderEvents():
		if got.ExchangeID != "ex-2" || got.ClientID != "local-2" || got.Status != models.OrderStatusFilled {
			t.Fatalf("order event = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for order event")
	}
	bal, err := sm.Balance(ctx, "ETH")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal.Total != "3" || bal.Available != "2.5" {
		t.Fatalf("ETH balance = %+v", bal)
	}
}

func TestSpotRiskContextAndBalanceChecks(t *testing.T) {
	ctx := context.Background()
	ex := newStubSpotExchange()
	sm, err := NewSpotMarket(ctx, SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatalf("NewSpotMarket: %v", err)
	}
	if err := sm.proj.Start(ctx); err != nil {
		t.Fatalf("projection start: %v", err)
	}
	tickerEv, err := spotex.NewPublicEvent(spotex.PublicTicker, &spotex.TickerSnapshot{Pair: "BTC/USDT", Last: "50000"})
	if err != nil {
		t.Fatal(err)
	}
	sm.router.OnPublicEvent(tickerEv)

	price := "51000"
	order := &models.Order{
		ID:        "sell-1",
		Market:    models.MarketSpot,
		Symbol:    "BTC/USDT",
		Side:      models.OrderSideSell,
		OrderType: models.OrderTypeLimit,
		Price:     &price,
		Size:      "0.1",
	}
	rc, err := sm.BuildRiskContext(ctx, "s1", &models.Signal{Symbol: "BTC/USDT", Direction: models.DirectionFlat}, order)
	if err != nil {
		t.Fatalf("BuildRiskContext: %v", err)
	}
	if rc.OpensExposure || rc.IncreasingExposure {
		t.Fatalf("sell should reduce exposure: %+v", rc)
	}
	if !rc.OrderNotional.Equal(decimalRequire("5100")) || !rc.PositionNotional.Equal(decimalRequire("25000")) || !rc.AccountEquity.Equal(decimalRequire("25100")) {
		t.Fatalf("notional/equity = order %s position %s equity %s", rc.OrderNotional, rc.PositionNotional, rc.AccountEquity)
	}

	buy := &models.Order{
		ID:        "buy-1",
		Market:    models.MarketSpot,
		Symbol:    "BTC/USDT",
		Side:      models.OrderSideBuy,
		OrderType: models.OrderTypeMarket,
		Size:      "1000",
	}
	if _, err := sm.BuildRiskContext(ctx, "s1", &models.Signal{Symbol: "BTC/USDT", Direction: models.DirectionLong}, buy); err == nil {
		t.Fatal("expected quote balance error")
	}
}

func decimalRequire(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}
