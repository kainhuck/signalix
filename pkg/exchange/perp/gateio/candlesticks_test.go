package gateio

import (
	"testing"

	"github.com/gate/gateapi-go/v7"
)

func TestCandlestickFromREST(t *testing.T) {
	row := &gateapi.FuturesCandlestick{
		T:   1539852480,
		O:   "1.0",
		H:   "1.1",
		L:   "0.9",
		C:   "1.05",
		V:   "97151",
		Sum: "3580",
	}
	got := candlestickFromREST("BTC_USDT", "5m", row)
	if got == nil {
		t.Fatal("nil snap")
	}
	if got.Contract != "BTC/USDT" || got.Interval != "5m" {
		t.Fatalf("contract/interval: %+v", got)
	}
	if got.TimestampSec != 1539852480 || !got.WindowClosed {
		t.Fatalf("time/closed: %+v", got)
	}
	if got.Close != "1.05" || got.Volume != "97151" || got.VolumeBase != "3580" {
		t.Fatalf("ohlcv: %+v", got)
	}
}

func TestCandlestickFromREST_nilTimestamp(t *testing.T) {
	if candlestickFromREST("BTC_USDT", "1m", &gateapi.FuturesCandlestick{T: 0}) != nil {
		t.Fatal("expected nil for zero timestamp")
	}
}
