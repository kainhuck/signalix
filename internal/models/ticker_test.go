package models

import (
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestTickerFromSnapshot(t *testing.T) {
	t.Parallel()

	snap := &perp.TickerSnapshot{
		Contract:        "BTC/USDT",
		Last:            "100",
		MarkPrice:       "100.1",
		TimestampMillis: 1716200000123,
	}
	ticker := TickerFromSnapshot(snap)
	if ticker.Contract != "BTC/USDT" || ticker.Last != "100" || ticker.TimestampMillis != 1716200000123 {
		t.Fatalf("unexpected ticker: %+v", ticker)
	}
}
