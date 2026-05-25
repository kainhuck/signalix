package models

import (
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestKlinesFromSnapshots(t *testing.T) {
	t.Parallel()

	snaps := []*perp.CandlestickSnapshot{
		{Contract: "BTC/USDT", Interval: "5m", Close: "1", WindowClosed: false},
	}
	bars := KlinesFromSnapshots(snaps)
	if len(bars) != 1 || !bars[0].WindowClosed {
		t.Fatalf("expected window_closed true, got %+v", bars[0])
	}
}
