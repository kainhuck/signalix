package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

type historyExchange struct {
	snaps map[perp.Contract][]*perp.CandlestickSnapshot
}

func (h *historyExchange) Ping(context.Context) error  { return nil }
func (h *historyExchange) Close(context.Context) error { return nil }
func (h *historyExchange) Connect(context.Context, perp.ConnectParts) error {
	return nil
}
func (h *historyExchange) ListContractMeta(context.Context) ([]*perp.ContractMeta, error) {
	return nil, nil
}
func (h *historyExchange) ListCandlesticks(_ context.Context, q *perp.ListCandlesticksQuery) ([]*perp.CandlestickSnapshot, error) {
	if q == nil {
		return nil, nil
	}
	return h.snaps[q.Contract], nil
}
func (h *historyExchange) Subscribe(context.Context, []*perp.Subscription) error { return nil }
func (h *historyExchange) Unsubscribe(context.Context, []*perp.Subscription) error {
	return nil
}
func (h *historyExchange) PublicEvents() <-chan *perp.PublicEvent {
	ch := make(chan *perp.PublicEvent)
	return ch
}
func (h *historyExchange) Place(context.Context, *perp.PlaceRequest) (*perp.OrderSnapshot, error) {
	return nil, nil
}
func (h *historyExchange) Cancel(context.Context, *perp.CancelParams) error { return nil }
func (h *historyExchange) GetOrder(context.Context, perp.Contract, string) (*perp.OrderSnapshot, error) {
	return nil, nil
}
func (h *historyExchange) Positions(context.Context) ([]*perp.PositionSnapshot, error) {
	return nil, nil
}
func (h *historyExchange) Position(context.Context, perp.Contract) (*perp.PositionSnapshot, error) {
	return nil, nil
}
func (h *historyExchange) Balance(context.Context) (*perp.BalanceView, error) {
	return nil, nil
}
func (h *historyExchange) UserEvents() <-chan *perp.UserEvent {
	ch := make(chan *perp.UserEvent)
	return ch
}

type historyRuntime struct {
	recordingStrategyRuntime
	lastHistory *models.HistoryPayload
}

func (r *historyRuntime) SendHistory(p *models.HistoryPayload) error {
	r.lastHistory = p
	return nil
}

func TestWarmupHistorySendHistory(t *testing.T) {
	t.Parallel()

	ex := &historyExchange{
		snaps: map[perp.Contract][]*perp.CandlestickSnapshot{
			"BTC/USDT": {
				{Contract: "BTC/USDT", Interval: "5m", Close: "1", TimestampSec: 1},
				{Contract: "BTC/USDT", Interval: "5m", Close: "2", TimestampSec: 2},
			},
		},
	}
	rec := &historyRuntime{recordingStrategyRuntime: recordingStrategyRuntime{name: "s1", ctx: context.Background()}}
	e := &Engine{
		ctx:      context.Background(),
		exchange: ex,
		router:   market.NewMarketRouter(ex),
	}
	st := &strategy.Strategy{
		StrategyConfig: strategy.StrategyConfig{
			Name:        "s1",
			Symbols:     []perp.Contract{"BTC/USDT"},
			HistoryBars: 100,
		},
	}

	if err := e.warmupHistory(st, rec, "5m"); err != nil {
		t.Fatal(err)
	}
	if rec.lastHistory == nil {
		t.Fatal("expected SendHistory")
	}
	if rec.lastHistory.Interval != "5m" || len(rec.lastHistory.Series) != 1 {
		t.Fatalf("unexpected payload: %+v", rec.lastHistory)
	}
	if len(rec.lastHistory.Series[0].Bars) != 2 {
		t.Fatalf("bars = %d, want 2", len(rec.lastHistory.Series[0].Bars))
	}
	if !rec.lastHistory.Series[0].Bars[0].WindowClosed {
		t.Fatal("expected window_closed true on history bars")
	}
}

func TestWarmupHistorySkipsWhenZero(t *testing.T) {
	t.Parallel()

	rec := &historyRuntime{recordingStrategyRuntime: recordingStrategyRuntime{ctx: context.Background()}}
	e := &Engine{ctx: context.Background(), exchange: &historyExchange{}}
	st := &strategy.Strategy{StrategyConfig: strategy.StrategyConfig{HistoryBars: 0}}

	if err := e.warmupHistory(st, rec, "5m"); err != nil {
		t.Fatal(err)
	}
	if rec.lastHistory != nil {
		t.Fatal("expected no history")
	}
}
