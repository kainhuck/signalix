package engine

import (
	"errors"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestTickerSnapshot_marketNotRegistered(t *testing.T) {
	t.Parallel()
	e := &Engine{}
	_, err := e.TickerSnapshot("BTC/USDT")
	if err == nil || err.Error() != `market "perp" not registered` {
		t.Fatalf("err = %v", err)
	}
}

func TestTickerSnapshot_notInCache(t *testing.T) {
	t.Parallel()
	e := &Engine{markets: testPerpMarkets(t, testutil.NewStubExchange())}
	_, err := e.TickerSnapshot("BTC/USDT")
	if !errors.Is(err, ErrTickerNotInCache) {
		t.Fatalf("err = %v", err)
	}
}

func TestTickerSnapshot_ok(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	markets := testPerpMarkets(t, ex)
	pm := testPerpFromMarkets(t, markets)
	router := pm.Router()
	e := &Engine{markets: markets}
	tick, _ := perp.NewPublicEvent(perp.PublicTicker, &perp.TickerSnapshot{
		Contract:        "BTC/USDT",
		Last:            "42000",
		TimestampMillis: 1000,
	})
	router.OnPublicEvent(tick)

	snap, err := e.TickerSnapshot("BTC/USDT")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Last != "42000" {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestClosedKlines_empty(t *testing.T) {
	t.Parallel()
	e := &Engine{markets: testPerpMarkets(t, testutil.NewStubExchange())}
	kl, err := e.ClosedKlines("BTC/USDT", "5m", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kl) != 0 {
		t.Fatalf("kl = %+v", kl)
	}
}

func TestClosedKlines_ok(t *testing.T) {
	t.Parallel()
	markets := testPerpMarkets(t, testutil.NewStubExchange())
	pm := testPerpFromMarkets(t, markets)
	router := pm.Router()
	e := &Engine{markets: markets}
	router.IngestHistoryKlines("5m", []*models.KlineSeries{{
		Contract: "BTC/USDT",
		Bars: []*models.Kline{
			{Contract: "BTC/USDT", Interval: "5m", Close: "1", TimestampSec: 1, WindowClosed: true},
			{Contract: "BTC/USDT", Interval: "5m", Close: "2", TimestampSec: 2, WindowClosed: true},
		},
	}})
	kl, err := e.ClosedKlines("BTC/USDT", "5m", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kl) != 2 || kl[1].Close != "2" {
		t.Fatalf("kl = %+v", kl)
	}
}

func TestListCachedTickers(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	markets := testPerpMarkets(t, ex)
	pm := testPerpFromMarkets(t, markets)
	router := pm.Router()
	e := &Engine{markets: markets}
	tick, _ := perp.NewPublicEvent(perp.PublicTicker, &perp.TickerSnapshot{
		Contract: "BTC/USDT",
		Last:     "1",
	})
	router.OnPublicEvent(tick)

	all, err := e.ListCachedTickers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all["BTC/USDT"].Last != "1" {
		t.Fatalf("all = %+v", all)
	}
}
