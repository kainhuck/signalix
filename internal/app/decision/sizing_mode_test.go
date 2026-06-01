package decision

import (
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/shopspring/decimal"
)

func TestComputeUSDTNotionalFromSignal_NilModeUsesDivisor(t *testing.T) {
	t.Parallel()
	sig := &models.Signal{Direction: models.DirectionLong}
	avail := decimal.NewFromInt(1000)
	got, err := ComputeUSDTNotionalFromSignal(sig, avail, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.NewFromInt(100)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestComputeUSDTNotionalFromSignal_NilModeDefaultDivisor(t *testing.T) {
	t.Parallel()
	sig := &models.Signal{Direction: models.DirectionLong}
	avail := decimal.NewFromInt(100)
	got, err := ComputeUSDTNotionalFromSignal(sig, avail, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.NewFromInt(10)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestComputeUSDTNotionalFromSignal_Percent(t *testing.T) {
	t.Parallel()
	mode := models.SizingModePercent
	val := "0.25"
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode, Value: &val}
	avail := decimal.NewFromInt(800)
	got, err := ComputeUSDTNotionalFromSignal(sig, avail, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.NewFromInt(200)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestComputeUSDTNotionalFromSignal_Fixed(t *testing.T) {
	t.Parallel()
	mode := models.SizingModeFixed
	val := "50"
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode, Value: &val}
	avail := decimal.NewFromInt(200)
	got, err := ComputeUSDTNotionalFromSignal(sig, avail, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.NewFromInt(50)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestComputeUSDTNotionalFromSignal_PercentMissingValue(t *testing.T) {
	t.Parallel()
	mode := models.SizingModePercent
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode}
	_, err := ComputeUSDTNotionalFromSignal(sig, decimal.NewFromInt(100), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestComputeUSDTNotionalFromSignal_PercentOutOfRange(t *testing.T) {
	t.Parallel()
	mode := models.SizingModePercent
	val := "1.5"
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode, Value: &val}
	_, err := ComputeUSDTNotionalFromSignal(sig, decimal.NewFromInt(100), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestComputeUSDTNotionalFromSignal_FixedExceedsAvailable(t *testing.T) {
	t.Parallel()
	mode := models.SizingModeFixed
	val := "500"
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode, Value: &val}
	_, err := ComputeUSDTNotionalFromSignal(sig, decimal.NewFromInt(100), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestComputeUSDTNotionalFromSignal_CustomRejected(t *testing.T) {
	t.Parallel()
	mode := models.SizingModeCustom
	val := "3"
	sig := &models.Signal{Direction: models.DirectionLong, SizingMode: &mode, Value: &val}
	_, err := ComputeUSDTNotionalFromSignal(sig, decimal.NewFromInt(100), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}
