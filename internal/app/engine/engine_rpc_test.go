package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestEngine_rpcGetBalanceAndPosition(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{{
		Contract:   "ETH/USDT",
		Side:       perp.PositionLong,
		Size:       "2",
		EntryPrice: "100",
		UpdatedAt:  time.Unix(1, 0),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	markets := testPerpMarkets(t, ex)
	pm := testPerpFromMarkets(t, markets)
	proj := pm.Projection()
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		proj.Stop()
	}()

	e := &Engine{markets: markets, accountProjection: proj}

	bal, err := e.rpcGetBalance()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := bal.(map[string]interface{})
	if !ok || m["available"] != "10000" {
		t.Fatalf("balance: %+v", bal)
	}

	pos, err := e.rpcGetPosition("ETH/USDT")
	if err != nil {
		t.Fatal(err)
	}
	pmOut, ok := pos.(map[string]interface{})
	if !ok || pmOut["size"] != "2" {
		t.Fatalf("position: %+v", pos)
	}

	none, err := e.rpcGetPosition("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Fatalf("want nil position, got %+v", none)
	}
}

func TestEngine_rpcGetKlines(t *testing.T) {
	ex := testutil.NewStubExchange()
	markets := testPerpMarkets(t, ex)
	pm := testPerpFromMarkets(t, markets)
	router := pm.Router()
	e := &Engine{markets: markets}
	e.setStrategyInterval("s1", "5m")

	router.IngestHistoryKlines("5m", []*models.KlineSeries{{
		Contract: "BTC/USDT",
		Bars: []*models.Kline{
			{Contract: "BTC/USDT", Interval: "5m", Close: "1", TimestampSec: 1, WindowClosed: true},
			{Contract: "BTC/USDT", Interval: "5m", Close: "2", TimestampSec: 2, WindowClosed: true},
		},
	}})

	out, err := e.rpcGetKlines("s1", map[string]interface{}{
		"symbol": "BTC/USDT",
		"limit":  float64(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	kl, ok := out.([]*models.Kline)
	if !ok || len(kl) != 2 {
		t.Fatalf("got %#v", out)
	}
}

func TestEngine_rpcGetTicker_ok(t *testing.T) {
	ex := testutil.NewStubExchange()
	markets := testPerpMarkets(t, ex)
	pm := testPerpFromMarkets(t, markets)
	router := pm.Router()
	e := &Engine{markets: markets}
	tick, err := perp.NewPublicEvent(perp.PublicTicker, &perp.TickerSnapshot{
		Contract:        "BTC/USDT",
		Last:            "42000",
		MarkPrice:       "42001",
		TimestampMillis: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	router.OnPublicEvent(tick)

	out, err := e.rpcGetTicker("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok || m["contract"] != "BTC/USDT" || m["last"] != "42000" || m["mark_price"] != "42001" {
		t.Fatalf("ticker: %+v", out)
	}
	if m["timestamp_millis"] != int64(1000) {
		t.Fatalf("timestamp_millis = %v", m["timestamp_millis"])
	}
}

func TestEngine_rpcGetTicker_notInCache(t *testing.T) {
	e := &Engine{markets: testPerpMarkets(t, testutil.NewStubExchange())}
	_, err := e.rpcGetTicker("BTC/USDT")
	if !errors.Is(err, ErrTickerNotInCache) {
		t.Fatalf("err = %v", err)
	}
}

func TestEngine_rpcGetTicker_symbolRequired(t *testing.T) {
	e := &Engine{markets: testPerpMarkets(t, testutil.NewStubExchange())}
	_, err := e.rpcGetTicker("")
	if err == nil || err.Error() != "symbol is required" {
		t.Fatalf("err = %v", err)
	}
}

func TestEngine_rpcGetTicker_marketNotRegistered(t *testing.T) {
	e := &Engine{}
	_, err := e.rpcGetTicker("BTC/USDT")
	if err == nil || err.Error() != `market "perp" not registered` {
		t.Fatalf("err = %v", err)
	}
}
