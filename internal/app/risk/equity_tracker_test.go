package risk

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestEquityTrackerDailyLossAndDrawdown(t *testing.T) {
	t.Parallel()
	tr := NewEquityTracker()
	base := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)

	tr.Init(decimal.NewFromInt(10000), base)
	if loss := tr.DailyLoss(); !loss.IsZero() {
		t.Fatalf("initial daily loss = %s", loss)
	}

	tr.OnEquityUpdate(decimal.NewFromInt(9000), base.Add(time.Hour))
	if loss := tr.DailyLoss(); !loss.Equal(decimal.NewFromInt(1000)) {
		t.Fatalf("daily loss = %s, want 1000", loss)
	}
	if dd := tr.DrawdownRatio(); !dd.Equal(decimal.RequireFromString("0.1")) {
		t.Fatalf("drawdown = %s", dd)
	}
}

func TestEquityTrackerUTCDayRollover(t *testing.T) {
	t.Parallel()
	tr := NewEquityTracker()
	d1 := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	d1Later := time.Date(2026, 5, 25, 18, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 26, 1, 0, 0, 0, time.UTC)

	tr.Init(decimal.NewFromInt(10000), d1)
	tr.OnEquityUpdate(decimal.NewFromInt(8000), d1Later)
	if loss := tr.DailyLoss(); !loss.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("day1 loss = %s", loss)
	}

	tr.OnEquityUpdate(decimal.NewFromInt(7500), d2)
	if loss := tr.DailyLoss(); !loss.IsZero() {
		t.Fatalf("day2 first sample loss = %s, want 0", loss)
	}
	tr.OnEquityUpdate(decimal.NewFromInt(7000), d2.Add(time.Hour))
	if loss := tr.DailyLoss(); !loss.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("day2 loss = %s, want 500", loss)
	}
}

func TestEquityTrackerSessionPeakPersistsAcrossDays(t *testing.T) {
	t.Parallel()
	tr := NewEquityTracker()
	d1 := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

	tr.Init(decimal.NewFromInt(10000), d1)
	tr.OnEquityUpdate(decimal.NewFromInt(12000), d1.Add(time.Hour))
	tr.OnEquityUpdate(decimal.NewFromInt(9000), d2)
	if peak := tr.SessionPeak(); !peak.Equal(decimal.NewFromInt(12000)) {
		t.Fatalf("session peak = %s", peak)
	}
}
