package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

type recordingStrategyRuntime struct {
	name       string
	ctx        context.Context
	lastKline  *models.Kline
	lastTrace  string
	sendKlineN int
}

func (r *recordingStrategyRuntime) Name() string                                     { return r.name }
func (r *recordingStrategyRuntime) Context() context.Context                         { return r.ctx }
func (r *recordingStrategyRuntime) SendInit(*strategy.Strategy) error                { return nil }
func (r *recordingStrategyRuntime) SendTick(*models.Ticker, string) error            { return nil }
func (r *recordingStrategyRuntime) SendStop() error                                  { return nil }
func (r *recordingStrategyRuntime) ReadMessages(func(strategy.IpcMessage)) error     { return nil }
func (r *recordingStrategyRuntime) ReadStderr()                                      {}
func (r *recordingStrategyRuntime) Stop() error                                      { return nil }
func (r *recordingStrategyRuntime) UpdateHeartbeat()                                 {}
func (r *recordingStrategyRuntime) GetLastHeartbeat() time.Time                      { return time.Now() }
func (r *recordingStrategyRuntime) RecordCrash()                                     {}
func (r *recordingStrategyRuntime) GetCrashCount() int                               { return 0 }
func (r *recordingStrategyRuntime) IsRunning() bool                                  { return true }
func (r *recordingStrategyRuntime) SendRPCResponse(string, interface{}, error) error { return nil }

func (r *recordingStrategyRuntime) SendKline(kline *models.Kline, traceID string) error {
	r.sendKlineN++
	r.lastKline = kline
	r.lastTrace = traceID
	return nil
}

func (r *recordingStrategyRuntime) SendHistory(*models.HistoryPayload) error { return nil }

func TestDispatchKlineUpdateSendKline(t *testing.T) {
	t.Parallel()

	rec := &recordingStrategyRuntime{name: "s1", ctx: context.Background()}
	e := &Engine{
		ctx:             context.Background(),
		strategyProcess: map[string]strategy.StrategyRuntime{"s1": rec},
	}

	e.dispatchKlineUpdate(market.MarketUpdate{
		StrategyName: "s1",
		Kind:         market.MarketUpdateKline,
		Kline: &perp.CandlestickSnapshot{
			Contract:     "BTC/USDT",
			Interval:     "1m",
			Close:        "100",
			TimestampSec: 1716200000,
			WindowClosed: true,
		},
	})

	if rec.sendKlineN != 1 {
		t.Fatalf("SendKline calls = %d, want 1", rec.sendKlineN)
	}
	if rec.lastKline == nil || rec.lastKline.Close != "100" || rec.lastKline.Interval != "1m" {
		t.Fatalf("unexpected kline: %+v", rec.lastKline)
	}
	if rec.lastTrace == "" {
		t.Fatal("expected trace id")
	}
}
