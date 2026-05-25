package market

import (
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestListClosedKlines_appendAndLimit(t *testing.T) {
	t.Parallel()
	mr := NewMarketRouter(newRecordingExchange())
	for i := int64(1); i <= 5; i++ {
		snap := &perp.CandlestickSnapshot{
			Contract:     "BTC/USDT",
			Interval:     "1m",
			Close:        "1",
			TimestampSec: i,
			WindowClosed: true,
		}
		ev, err := perp.NewPublicEvent(perp.PublicCandlestick, snap)
		if err != nil {
			t.Fatal(err)
		}
		mr.OnPublicEvent(ev)
	}
	got := mr.ListClosedKlines("BTC/USDT", "1m", 3)
	if len(got) != 3 {
		t.Fatalf("len %d want 3", len(got))
	}
	if got[0].TimestampSec != 3 || got[2].TimestampSec != 5 {
		t.Fatalf("order: %d %d %d", got[0].TimestampSec, got[1].TimestampSec, got[2].TimestampSec)
	}
}

func TestIngestHistoryKlines_merge(t *testing.T) {
	t.Parallel()
	mr := NewMarketRouter(newRecordingExchange())
	mr.IngestHistoryKlines("5m", []*models.KlineSeries{{
		Contract: "ETH/USDT",
		Bars: []*models.Kline{
			{Contract: "ETH/USDT", Interval: "5m", Close: "1", TimestampSec: 10, WindowClosed: true},
			{Contract: "ETH/USDT", Interval: "5m", Close: "2", TimestampSec: 20, WindowClosed: true},
		},
	}})
	got := mr.ListClosedKlines("ETH/USDT", "5m", 10)
	if len(got) != 2 || got[0].Close != "1" {
		t.Fatalf("got %+v", got)
	}
}
