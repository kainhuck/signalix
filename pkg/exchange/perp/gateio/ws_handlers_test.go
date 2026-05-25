package gateio

import (
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestParseCandlestickName(t *testing.T) {
	interval, contract := parseCandlestickName("1m_BTC_USDT")
	if interval != "1m" || contract != "BTC/USDT" {
		t.Fatalf("got interval=%q contract=%q", interval, contract)
	}
	interval, contract = parseCandlestickName("10s_ETH_USD")
	if interval != "10s" || contract != "ETH/USD" {
		t.Fatalf("got interval=%q contract=%q", interval, contract)
	}
}

func TestBalanceUpdateFromWSRow(t *testing.T) {
	snap, ok := balanceUpdateFromWSRow(map[string]interface{}{
		"balance":  9.998739899488,
		"change":   -0.000002074115,
		"text":     "BTC_USD:3914424",
		"time_ms":  float64(1547199246123),
		"type":     "fee",
		"user":     "211xxx",
		"currency": "usdt",
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Currency != "USDT" || snap.ChangeType != "fee" || snap.Balance == "" {
		t.Fatalf("snap: %+v", snap)
	}
}

func TestPositionFromWSRow(t *testing.T) {
	snap, ok := positionFromWSRow(map[string]interface{}{
		"contract":    "BTC_USDT",
		"entry_price": 40000.36666661111,
		"leverage":    float64(10),
		"size":        "3",
		"time_ms":     float64(1628736848321),
		"user":        "110xxxxx",
		"update_id":   float64(170919),
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Contract != "BTC/USDT" || snap.Side != perp.PositionLong || snap.Size != "3" ||
		snap.EntryPrice == "" || snap.Leverage != 10 {
		t.Fatalf("snap: %+v", snap)
	}
}

func TestPositionFromWSRow_Closed(t *testing.T) {
	snap, ok := positionFromWSRow(map[string]interface{}{
		"contract": "ETH_USDT",
		"size":     "0",
	})
	if !ok || snap.Size != "0" {
		t.Fatalf("snap: %+v ok=%v", snap, ok)
	}
}

func TestPositionFromWSRow_Short(t *testing.T) {
	snap, ok := positionFromWSRow(map[string]interface{}{
		"contract": "BTC_USDT",
		"size":     "-2",
	})
	if !ok || snap.Side != perp.PositionShort || snap.Size != "2" {
		t.Fatalf("snap: %+v ok=%v", snap, ok)
	}
}

func TestUserTradeFromWSRow(t *testing.T) {
	snap, ok := userTradeFromWSRow(map[string]interface{}{
		"id":             "3335259",
		"create_time":    float64(1628736848),
		"create_time_ms": float64(1628736848321),
		"contract":       "BTC_USDT",
		"order_id":       "4872460",
		"size":           "1",
		"price":          "40000.4",
		"role":           "maker",
		"text":           "api",
		"fee":            0.0009290592,
		"point_fee":      float64(0),
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Contract != "BTC/USDT" || snap.TradeID != "3335259" || snap.Side != perp.SideBuy ||
		snap.Size != "1" || snap.Price != "40000.4" || snap.Role != "maker" || snap.Text != "api" {
		t.Fatalf("snap: %+v", snap)
	}
	if snap.ExchangeOrderID != "4872460" || snap.Fee == "" {
		t.Fatalf("order/fee: %+v", snap)
	}
}

func TestUserTradeFromWSRow_SellSize(t *testing.T) {
	snap, ok := userTradeFromWSRow(map[string]interface{}{
		"id":       "1",
		"contract": "ETH_USDT",
		"size":     "-2",
		"price":    "3000",
	})
	if !ok || snap.Side != perp.SideSell || snap.Size != "2" {
		t.Fatalf("snap: %+v ok=%v", snap, ok)
	}
}

func TestCandlestickFromWSRow(t *testing.T) {
	snap, ok := candlestickFromWSRow(map[string]interface{}{
		"t": float64(1545129300),
		"o": "94.3",
		"h": "96.9",
		"l": "89.5",
		"c": "95.4",
		"v": "27525555",
		"n": "1m_BTC_USDT",
		"a": "314732.87412",
		"w": false,
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Interval != "1m" || snap.Contract != "BTC/USDT" || snap.Close != "95.4" || snap.WindowClosed {
		t.Fatalf("snap: %+v", snap)
	}
	if snap.TimestampSec != 1545129300 {
		t.Fatalf("timestamp: %d", snap.TimestampSec)
	}
}
