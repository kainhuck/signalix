package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

type dispatchTestMarket struct {
	kind models.Market

	mu             sync.Mutex
	subscribeCalls []string
	decideFn       func(strategy string, sig *models.Signal) (*models.Order, error)
	updates        chan market.MarketUpdate
	orderEvents    chan *models.OrderEvent
}

func newDispatchTestMarket(kind models.Market) *dispatchTestMarket {
	return &dispatchTestMarket{
		kind:        kind,
		updates:     make(chan market.MarketUpdate, 8),
		orderEvents: make(chan *models.OrderEvent, 4),
	}
}

func (m *dispatchTestMarket) Kind() models.Market         { return m.kind }
func (m *dispatchTestMarket) Start(context.Context) error { return nil }
func (m *dispatchTestMarket) Stop() error                 { return nil }

func (m *dispatchTestMarket) Subscribe(_ context.Context, req market.SubscribeRequest) error {
	m.mu.Lock()
	m.subscribeCalls = append(m.subscribeCalls, req.Strategy)
	m.mu.Unlock()
	return nil
}

func (m *dispatchTestMarket) Unsubscribe(string) error { return nil }

func (m *dispatchTestMarket) WarmupHistory(context.Context, market.SubscribeRequest, int) (*models.HistoryPayload, error) {
	return nil, nil
}

func (m *dispatchTestMarket) Updates() <-chan market.MarketUpdate { return m.updates }

func (m *dispatchTestMarket) Decide(_ context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	if m.decideFn != nil {
		return m.decideFn(strategy, sig)
	}
	return nil, nil
}

func (m *dispatchTestMarket) Place(context.Context, *models.Order) (string, error) { return "", nil }
func (m *dispatchTestMarket) Cancel(context.Context, *models.Order) error          { return nil }
func (m *dispatchTestMarket) Sync(context.Context, *models.Order) (*models.OrderEvent, error) {
	return nil, nil
}
func (m *dispatchTestMarket) OrderEvents() <-chan *models.OrderEvent { return m.orderEvents }

func (m *dispatchTestMarket) BuildRiskContext(_ context.Context, strategyName string, signal *models.Signal, order *models.Order) (*ports.RiskContext, error) {
	return &ports.RiskContext{
		StrategyName:    strategyName,
		Signal:          signal,
		Order:           order,
		ProjectionReady: true,
		OpensExposure:   true,
		AccountEquity:   decimal.NewFromInt(100_000),
	}, nil
}

func (m *dispatchTestMarket) Balance(context.Context, string) (*models.BalanceView, error) {
	return nil, nil
}
func (m *dispatchTestMarket) Position(context.Context, string) (*models.PositionView, error) {
	return nil, nil
}
func (m *dispatchTestMarket) Ticker(string) (*models.Ticker, error) { return nil, nil }
func (m *dispatchTestMarket) Klines(string, string, int) ([]*models.Kline, error) {
	return nil, nil
}

var _ market.Market = (*dispatchTestMarket)(nil)

func TestPumpMarketUpdates_multiMarket(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	perpM := newDispatchTestMarket(models.MarketPerp)
	spotM := newDispatchTestMarket(models.MarketSpot)
	recPerp := &recordingStrategyRuntime{name: "s-perp", ctx: ctx}
	recSpot := &recordingStrategyRuntime{name: "s-spot", ctx: ctx}

	e := &Engine{
		ctx:             ctx,
		cancel:          cancel,
		markets:         map[models.Market]market.Market{models.MarketPerp: perpM, models.MarketSpot: spotM},
		strategyProcess: map[string]strategy.StrategyRuntime{"s-perp": recPerp, "s-spot": recSpot},
	}

	go e.pumpMarketUpdates(models.MarketPerp, perpM)
	go e.pumpMarketUpdates(models.MarketSpot, spotM)

	perpM.updates <- market.MarketUpdate{
		Market:       models.MarketPerp,
		StrategyName: "s-perp",
		Kind:         market.MarketUpdateKline,
		Kline:        &models.Kline{Contract: "BTC/USDT", Interval: "1m", Close: "1", WindowClosed: true},
	}
	spotM.updates <- market.MarketUpdate{
		Market:       models.MarketSpot,
		StrategyName: "s-spot",
		Kind:         market.MarketUpdateTicker,
		Ticker:       &models.Ticker{Contract: "ETH/USDT", Last: "2"},
	}

	deadline := time.After(500 * time.Millisecond)
	for {
		if recPerp.sendKlineN >= 1 && recSpot.sendTickN >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("perp kline=%d spot tick=%d", recPerp.sendKlineN, recSpot.sendTickN)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestSubscribeStrategy_routesByMarket(t *testing.T) {
	t.Parallel()

	perpM := newDispatchTestMarket(models.MarketPerp)
	spotM := newDispatchTestMarket(models.MarketSpot)
	e := &Engine{
		ctx:     context.Background(),
		markets: map[models.Market]market.Market{models.MarketPerp: perpM, models.MarketSpot: spotM},
	}

	if err := e.subscribeStrategy(&strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{Name: "alpha", Symbols: []perp.Contract{"BTC/USDT"}},
		Market:         models.MarketPerp,
	}, "1m"); err != nil {
		t.Fatal(err)
	}
	if err := e.subscribeStrategy(&strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{Name: "beta", Symbols: []perp.Contract{"ETH/USDT"}},
		Market:         models.MarketSpot,
	}, "5m"); err != nil {
		t.Fatal(err)
	}

	perpM.mu.Lock()
	perpCalls := append([]string(nil), perpM.subscribeCalls...)
	perpM.mu.Unlock()
	spotM.mu.Lock()
	spotCalls := append([]string(nil), spotM.subscribeCalls...)
	spotM.mu.Unlock()

	if len(perpCalls) != 1 || perpCalls[0] != "alpha" {
		t.Fatalf("perp subscribe = %v", perpCalls)
	}
	if len(spotCalls) != 1 || spotCalls[0] != "beta" {
		t.Fatalf("spot subscribe = %v", spotCalls)
	}
}

func TestDispatchSignal_routesDeciderByMarket(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	perpM := newDispatchTestMarket(models.MarketPerp)
	spotM := newDispatchTestMarket(models.MarketSpot)
	perpM.decideFn = func(_ string, _ *models.Signal) (*models.Order, error) {
		return &models.Order{ID: "perp-order", Symbol: "BTC/USDT", Size: "1"}, nil
	}
	spotM.decideFn = func(_ string, _ *models.Signal) (*models.Order, error) {
		return &models.Order{ID: "spot-order", Symbol: "ETH/USDT", Size: "2"}, nil
	}

	orderCh := make(chan *models.Order, 1)
	e := &Engine{
		ctx:     ctx,
		cancel:  cancel,
		markets: map[models.Market]market.Market{models.MarketPerp: perpM, models.MarketSpot: spotM},
		strategies: map[string]*strategy.Strategy{
			"spot-strat": {
				StrategyConfig: strategy.StrategyConfig{Name: "spot-strat"},
				Market:         models.MarketSpot,
			},
		},
		signalCh:        make(chan *models.StrategySignal, 1),
		orderCh:         orderCh,
		riskEvaluator:   NewStaticRiskEvaluator(risk.DefaultRules()),
		executionEngine: nil,
	}

	go e.dispatchSignal()

	e.signalCh <- &models.StrategySignal{
		StrategyName: "spot-strat",
		Signal:       &models.Signal{Symbol: "ETH/USDT", Direction: models.DirectionLong, Strength: 0.5},
	}

	select {
	case o := <-orderCh:
		if o.ID != "spot-order" {
			t.Fatalf("order id = %q", o.ID)
		}
		if o.Market != models.MarketSpot {
			t.Fatalf("order market = %q, want spot", o.Market)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for order")
	}
	cancel()
}

func TestDispatchSignal_singlePerpRegression(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	perpM := newDispatchTestMarket(models.MarketPerp)
	perpM.decideFn = func(_ string, _ *models.Signal) (*models.Order, error) {
		return &models.Order{ID: "ord-1", Symbol: "BTC/USDT", Size: "1"}, nil
	}

	orderCh := make(chan *models.Order, 1)
	e := &Engine{
		ctx:     ctx,
		cancel:  cancel,
		markets: map[models.Market]market.Market{models.MarketPerp: perpM},
		strategies: map[string]*strategy.Strategy{
			"s1": {
				StrategyConfig: strategy.StrategyConfig{Name: "s1"},
				Market:         models.MarketPerp,
			},
		},
		signalCh:      make(chan *models.StrategySignal, 1),
		orderCh:       orderCh,
		riskEvaluator: NewStaticRiskEvaluator(risk.DefaultRules()),
	}

	go e.dispatchSignal()

	e.signalCh <- &models.StrategySignal{
		StrategyName: "s1",
		Signal:       &models.Signal{Symbol: "BTC/USDT", Direction: models.DirectionLong, Strength: 0.5},
	}

	select {
	case o := <-orderCh:
		if o.ID != "ord-1" || o.Market != models.MarketPerp {
			t.Fatalf("unexpected order: %+v", o)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for order")
	}
	cancel()
}

func TestStrategyMarket_defaultsPerp(t *testing.T) {
	t.Parallel()
	e := &Engine{
		strategies: map[string]*strategy.Strategy{
			"s1": {StrategyConfig: strategy.StrategyConfig{Name: "s1"}},
		},
	}
	if got := e.strategyMarket("s1"); got != models.MarketPerp {
		t.Fatalf("market = %q, want perp", got)
	}
}
