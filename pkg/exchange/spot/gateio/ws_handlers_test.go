package gateio

import (
	"testing"
	"time"
)

func TestHandleTicker(t *testing.T) {
	c := NewClient("k", "s")
	h := newWSHub(c)
	msg := map[string]interface{}{
		"event":   "update",
		"channel": WSChannelTickers,
		"time_ms": float64(1700000000000),
		"result": map[string]interface{}{
			"currency_pair":     "BTC_USDT",
			"last":              "50000",
			"highest_bid":       "49999",
			"lowest_ask":        "50001",
			"change_percentage": "2.5",
			"base_volume":       "100",
			"quote_volume":      "5000000",
			"high_24h":          "51000",
			"low_24h":           "49000",
		},
	}
	h.handleTicker(msg)
	select {
	case ev := <-c.pubCh:
		snap, ok := ev.Ticker()
		if !ok {
			t.Fatal("expected ticker event")
		}
		if snap.Pair != "BTC/USDT" || snap.Last != "50000" {
			t.Fatalf("snap=%+v", snap)
		}
		if snap.ChangePct24h != "0.025" {
			t.Fatalf("change=%q", snap.ChangePct24h)
		}
	default:
		t.Fatal("no event")
	}
}

func TestCandlestickFromWSRow(t *testing.T) {
	snap, ok := candlestickFromWSRow(map[string]interface{}{
		"t": float64(1700000000),
		"o": "1",
		"h": "2",
		"l": "0.5",
		"c": "1.5",
		"v": "100",
		"a": "50",
		"n": "1m_BTC_USDT",
		"w": true,
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Pair != "BTC/USDT" || snap.Interval != "1m" || !snap.WindowClosed {
		t.Fatalf("snap=%+v", snap)
	}
}

func TestHandleOrder(t *testing.T) {
	c := NewClient("k", "s")
	h := newWSHub(c)
	msg := map[string]interface{}{
		"event":   "update",
		"channel": WSChannelOrders,
		"result": map[string]interface{}{
			"id":             "123",
			"text":           "t-abc",
			"currency_pair":  "BTC_USDT",
			"side":           "buy",
			"type":           "limit",
			"amount":         "0.01",
			"price":          "50000",
			"left":           "0.01",
			"filled_amount":  "0",
			"status":         "open",
			"create_time_ms": float64(1700000000000),
		},
	}
	h.handleOrder(msg)
	select {
	case ev := <-c.userCh:
		ord, ok := ev.Order()
		if !ok || ord.OrderID != "123" || ord.Pair != "BTC/USDT" {
			t.Fatalf("ord=%+v", ord)
		}
	default:
		t.Fatal("no event")
	}
}

func TestBalanceUpdateFromWSRow(t *testing.T) {
	snap, ok := balanceUpdateFromWSRow(map[string]interface{}{
		"currency":     "USDT",
		"total":        "100",
		"available":    "80",
		"freeze":       "20",
		"change":       "10",
		"change_type":  "order-match",
		"timestamp_ms": float64(1700000000000),
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if snap.Currency != "USDT" || snap.Total != "100" || snap.Frozen != "20" {
		t.Fatalf("snap=%+v", snap)
	}
	if snap.UpdatedAt.UnixMilli() != 1700000000000 {
		t.Fatalf("updatedAt=%v", snap.UpdatedAt)
	}
}

func TestHandleBalances_array(t *testing.T) {
	c := NewClient("k", "s")
	h := newWSHub(c)
	msg := map[string]interface{}{
		"event":   "update",
		"channel": WSChannelBalances,
		"result": []interface{}{
			map[string]interface{}{
				"currency":     "BTC",
				"available":    "1",
				"freeze":       "0.1",
				"timestamp_ms": float64(time.Now().UnixMilli()),
			},
		},
	}
	h.handleBalances(msg)
	select {
	case ev := <-c.userCh:
		bal, ok := ev.Balance()
		if !ok || bal.Currency != "BTC" {
			t.Fatalf("bal=%+v", bal)
		}
	default:
		t.Fatal("no event")
	}
}

func TestSpotOrderFromWSElement(t *testing.T) {
	ord, ok := spotOrderFromWSElement(map[string]interface{}{
		"id":            "1",
		"currency_pair": "ETH_USDT",
		"amount":        float64(1.5),
		"price":         "2000",
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if ord.Id != "1" || ord.Amount != "1.5" {
		t.Fatalf("ord=%+v", ord)
	}
}
