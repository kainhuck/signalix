package grpc

import (
	"context"
	"testing"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/market"
	mktspot "github.com/kainhuck/signalix/internal/app/market/spot"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

func testRunningSpotService(t *testing.T) (*EngineService, *mktspot.SpotMarket) {
	t.Helper()
	ex := testutil.NewStubSpotExchange()
	ex.BalancesList = append(ex.BalancesList, &spotex.BalanceView{
		Currency:  "BTC",
		Total:     "0.5",
		Available: "0.4",
		Frozen:    "0.1",
		UpdatedAt: time.Unix(10, 0),
	})
	sm, err := mktspot.NewSpotMarket(context.Background(), mktspot.SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.Projection().Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewEngine(t.TempDir(), map[models.Market]market.Market{models.MarketSpot: sm}, engine.BuildParams{})
	eng.SetRunning(true)
	return NewEngineService(eng), sm
}

func TestSpotAccountRPC(t *testing.T) {
	t.Parallel()
	svc, _ := testRunningSpotService(t)

	bal, err := svc.GetBalance(context.Background(), &enginev1.GetBalanceRequest{Market: "spot", Currency: "btc"})
	if err != nil {
		t.Fatal(err)
	}
	if bal.GetBalance().GetMarket() != "spot" || bal.GetBalance().GetCurrency() != "BTC" || bal.GetBalance().GetTotal() != "0.5" {
		t.Fatalf("balance = %+v", bal.GetBalance())
	}

	pos, err := svc.GetPosition(context.Background(), &enginev1.GetPositionRequest{Market: "spot", Symbol: "BTC/USDT"})
	if err != nil {
		t.Fatal(err)
	}
	if pos.GetPosition().GetMarket() != "spot" || pos.GetPosition().GetSymbol() != "BTC/USDT" || pos.GetPosition().GetSize() != "0.5" {
		t.Fatalf("position = %+v", pos.GetPosition())
	}
}

func TestSpotMarketRPC(t *testing.T) {
	t.Parallel()
	svc, sm := testRunningSpotService(t)
	tick, err := spotex.NewPublicEvent(spotex.PublicTicker, &spotex.TickerSnapshot{
		Pair: "BTC/USDT", Last: "66000", TimestampMillis: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Router().OnPublicEvent(tick)
	candle, err := spotex.NewPublicEvent(spotex.PublicCandlestick, &spotex.CandlestickSnapshot{
		Pair: "BTC/USDT", Interval: "1m", Open: "1", High: "2", Low: "1", Close: "1.5", Volume: "10", TimestampSec: 10, WindowClosed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Router().OnPublicEvent(candle)

	ticker, err := svc.GetTicker(context.Background(), &enginev1.GetTickerRequest{Market: "spot", Symbol: "btcusdt"})
	if err != nil {
		t.Fatal(err)
	}
	if ticker.GetTicker().GetMarket() != "spot" || ticker.GetTicker().GetSymbol() != "BTC/USDT" || ticker.GetTicker().GetLast() != "66000" {
		t.Fatalf("ticker = %+v", ticker.GetTicker())
	}

	klines, err := svc.GetKlines(context.Background(), &enginev1.GetKlinesRequest{Market: "spot", Symbol: "BTC/USDT", Interval: "1m"})
	if err != nil {
		t.Fatal(err)
	}
	if len(klines.GetKlines()) != 1 || klines.GetKlines()[0].GetMarket() != "spot" || klines.GetKlines()[0].GetClose() != "1.5" {
		t.Fatalf("klines = %+v", klines.GetKlines())
	}
}

func TestOrderProtoIncludesMarket(t *testing.T) {
	t.Parallel()
	out := orderToProto(&models.Order{
		ID:           "o-1",
		Market:       models.MarketSpot,
		Symbol:       perp.Contract("BTC/USDT"),
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeMarket,
		Size:         "10",
		Status:       models.OrderStatusSubmitted,
		StrategyName: "s1",
		CreatedAt:    time.Unix(1, 0),
		UpdatedAt:    time.Unix(2, 0),
	})
	if out.GetMarket() != "spot" {
		t.Fatalf("market = %q", out.GetMarket())
	}
}

func TestListOpenOrdersFiltersMarket(t *testing.T) {
	t.Parallel()
	eng := engine.NewEngine(t.TempDir(), nil, engine.BuildParams{})
	eng.ExecutionEngine().HydrateFromSnapshot([]*models.Order{
		{
			ID:        "perp-1",
			Market:    models.MarketPerp,
			Symbol:    perp.Contract("BTC/USDT"),
			Status:    models.OrderStatusSubmitted,
			UpdatedAt: time.Unix(1, 0),
		},
		{
			ID:        "spot-1",
			Market:    models.MarketSpot,
			Symbol:    perp.Contract("BTC/USDT"),
			Status:    models.OrderStatusSubmitted,
			UpdatedAt: time.Unix(2, 0),
		},
	})
	eng.SetRunning(true)
	svc := NewEngineService(eng)

	all, err := svc.ListOpenOrders(context.Background(), &enginev1.ListOpenOrdersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.GetOrders()) != 2 {
		t.Fatalf("all = %+v", all.GetOrders())
	}

	spotOnly, err := svc.ListOpenOrders(context.Background(), &enginev1.ListOpenOrdersRequest{Market: "spot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(spotOnly.GetOrders()) != 1 || spotOnly.GetOrders()[0].GetId() != "spot-1" || spotOnly.GetOrders()[0].GetMarket() != "spot" {
		t.Fatalf("spotOnly = %+v", spotOnly.GetOrders())
	}
}
