package gateio

import "testing"

func TestCandlestickFromREST(t *testing.T) {
	row := []string{"1539852480", "97151.064", "1.032", "1.033", "1.031", "1.03"}
	got := candlestickFromREST("BTC_USDT", "5m", row)
	if got == nil {
		t.Fatal("nil snap")
	}
	if got.Pair != "BTC/USDT" || got.Interval != "5m" {
		t.Fatalf("pair/interval: %+v", got)
	}
	if got.TimestampSec != 1539852480 || !got.WindowClosed {
		t.Fatalf("time/closed: %+v", got)
	}
	if got.Close != "1.032" || got.Open != "1.03" || got.Volume != "97151.064" {
		t.Fatalf("ohlcv: %+v", got)
	}
}

func TestCandlestickFromREST_shortRow(t *testing.T) {
	if candlestickFromREST("BTC_USDT", "1m", []string{"1", "2"}) != nil {
		t.Fatal("expected nil for short row")
	}
}
