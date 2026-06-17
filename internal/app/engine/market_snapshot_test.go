package engine

import (
	"errors"
	"testing"

	mktspot "github.com/kainhuck/signalix/internal/app/market/spot"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
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

func TestTickerSnapshotForMarket_spot(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubSpotExchange()
	markets := testSpotMarkets(t, ex)
	sm := markets[models.MarketSpot].(*mktspot.SpotMarket)
	e := &Engine{markets: markets}

	tick, err := spotex.NewPublicEvent(spotex.PublicTicker, &spotex.TickerSnapshot{
		Pair:            "BTC/USDT",
		Last:            "66000",
		TimestampMillis: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Router().OnPublicEvent(tick)

	snap, err := e.TickerSnapshotForMarket(models.MarketSpot, "btcusdt")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Contract != "BTC/USDT" || snap.Last != "66000" {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestClosedKlinesForMarket_spot(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubSpotExchange()
	markets := testSpotMarkets(t, ex)
	sm := markets[models.MarketSpot].(*mktspot.SpotMarket)
	e := &Engine{markets: markets}

	ev, err := spotex.NewPublicEvent(spotex.PublicCandlestick, &spotex.CandlestickSnapshot{
		Pair: "BTC/USDT", Interval: "1m", Open: "1", High: "2", Low: "1", Close: "1.5", Volume: "10", TimestampSec: 10, WindowClosed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Router().OnPublicEvent(ev)

	kl, err := e.ClosedKlinesForMarket(models.MarketSpot, "BTC/USDT", "1m", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kl) != 1 || kl[0].Close != "1.5" {
		t.Fatalf("kl = %+v", kl)
	}
}
