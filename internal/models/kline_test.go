package models

import (
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestKlineFromSnapshot(t *testing.T) {
	t.Parallel()

	snap := &perp.CandlestickSnapshot{
		Contract:     "BTC/USDT",
		Interval:     "5m",
		Open:         "1",
		High:         "2",
		Low:          "0.5",
		Close:        "1.5",
		Volume:       "100",
		TimestampSec: 1716200000,
		WindowClosed: true,
	}
	k := KlineFromSnapshot(snap)
	if k.Contract != snap.Contract || k.Interval != "5m" || k.Close != "1.5" || !k.WindowClosed {
		t.Fatalf("unexpected kline: %+v", k)
	}
}
