package gateio

import (
	"testing"

	"github.com/gate/gateapi-go/v7"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

func TestToGatePair(t *testing.T) {
	if got := toGatePair(spot.CanonicalPair("BTC/USDT")); got != "BTC_USDT" {
		t.Fatalf("got %q", got)
	}
}

func TestOrderViewFromSpot_limitBuy(t *testing.T) {
	o := gateapi.Order{
		Id:           "123",
		CurrencyPair: "BTC_USDT",
		Side:         "buy",
		Type:         "limit",
		Amount:       "0.01",
		Price:        "50000",
		Left:         "0.005",
		FilledAmount: "0.005",
		Status:       "open",
		Text:         "t-local-1",
		CreateTimeMs: 1_700_000_000_000,
	}
	got := orderViewFromSpot(o)
	if got.Pair != "BTC/USDT" || got.Side != spot.SideBuy {
		t.Fatalf("pair/side: %+v", got)
	}
	if got.Status != spot.OrderPartialFilled {
		t.Fatalf("status: %s", got.Status)
	}
	if got.FilledSize != "0.005" {
		t.Fatalf("filled: %s", got.FilledSize)
	}
	if got.ClientID != "t-local-1" {
		t.Fatalf("client id: %s", got.ClientID)
	}
}

func TestOrderViewFromSpot_cancelled(t *testing.T) {
	o := gateapi.Order{
		Id:           "1",
		CurrencyPair: "ETH_USDT",
		Side:         "sell",
		Type:         "market",
		Amount:       "1",
		Status:       "cancelled",
	}
	got := orderViewFromSpot(o)
	if got.Status != spot.OrderCancelled {
		t.Fatalf("status: %s", got.Status)
	}
}

func TestBuildGateOrder_marketBuy(t *testing.T) {
	qa := "100"
	req := &spot.PlaceRequest{
		Pair:        spot.CanonicalPair("BTC/USDT"),
		Side:        spot.SideBuy,
		Type:        spot.OrderTypeMarket,
		QuoteAmount: &qa,
	}
	ord, err := buildGateOrder(req)
	if err != nil {
		t.Fatal(err)
	}
	if ord.Amount != "100" || ord.Side != "buy" || ord.Type != "market" || ord.Account != "spot" {
		t.Fatalf("order: %+v", ord)
	}
}

func TestBuildGateOrder_limitSell(t *testing.T) {
	price := "50000"
	req := &spot.PlaceRequest{
		Pair:  spot.CanonicalPair("BTC/USDT"),
		Side:  spot.SideSell,
		Type:  spot.OrderTypeLimit,
		Size:  "0.01",
		Price: &price,
	}
	ord, err := buildGateOrder(req)
	if err != nil {
		t.Fatal(err)
	}
	if ord.Amount != "0.01" || ord.Price != "50000" || ord.TimeInForce != "gtc" {
		t.Fatalf("order: %+v", ord)
	}
}

func TestNormalizeClientOrderID(t *testing.T) {
	got := normalizeClientOrderID("my-order-id")
	if !stringsHasPrefix(got, "t-") {
		t.Fatalf("got %q", got)
	}
	if len(got) > 30 {
		t.Fatalf("too long: %q", got)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestPairMetaFromGate(t *testing.T) {
	pm := pairMetaFromGate(gateapi.CurrencyPair{
		Id:              "BTC_USDT",
		MinBaseAmount:   "0.0001",
		AmountPrecision: 4,
		Precision:       2,
		TradeStatus:     "tradable",
	})
	if pm == nil || pm.Pair != "BTC/USDT" || pm.TradeStatus != "tradable" {
		t.Fatalf("meta: %+v", pm)
	}
}
