package decision

import (
	"context"
	"strings"
	"testing"

	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

type stubTickerLookup struct {
	tickers map[perp.Contract]*perp.TickerSnapshot
}

func (s *stubTickerLookup) GetCachedTicker(c perp.Contract) (*perp.TickerSnapshot, bool) {
	t, ok := s.tickers[c]
	return t, ok
}

func testRegistryFromExchange(t *testing.T, ex perp.InstrumentSource) instrument.ContractMetaLookup {
	t.Helper()
	reg := instrument.NewRegistry()
	if err := reg.LoadFrom(context.Background(), ex); err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	return reg
}

func TestNotionalUSDTToContracts(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.ContractMetas = []*perp.ContractMeta{{
		Contract:          "BTC/USDT",
		QuantoMultiplier:  "0.0001",
		OrderSizeMin:      "1",
		EnableDecimalSize: false,
	}}
	lookup := &stubTickerLookup{tickers: map[perp.Contract]*perp.TickerSnapshot{
		"BTC/USDT": {Contract: "BTC/USDT", MarkPrice: "50000"},
	}}
	de := NewDecisionEngine(nil, WithContractMetaLookup(testRegistryFromExchange(t, ex)), WithTickerLookup(lookup))
	got, err := de.notionalUSDTToContracts(context.Background(), "BTC/USDT", decimal.NewFromInt(1000))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "200" {
		t.Fatalf("got %s want 200", got)
	}
}

func TestNotionalUSDTToContracts_noLookup(t *testing.T) {
	t.Parallel()
	lookup := &stubTickerLookup{tickers: map[perp.Contract]*perp.TickerSnapshot{
		"BTC/USDT": {Contract: "BTC/USDT", MarkPrice: "50000"},
	}}
	de := NewDecisionEngine(nil, WithTickerLookup(lookup))
	_, err := de.notionalUSDTToContracts(context.Background(), "BTC/USDT", decimal.NewFromInt(1000))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "contract meta lookup not configured") {
		t.Fatalf("err = %v", err)
	}
}

func TestDecisionEngine_ProcessSignal_PercentUsesContracts(t *testing.T) {
	ex := testutil.NewStubExchange()
	ex.ContractMetas = []*perp.ContractMeta{{
		Contract:         "BTC/USDT",
		QuantoMultiplier: "0.0001",
		OrderSizeMin:     "1",
	}}
	ex.BalanceAvailable = "10000"
	lookup := &stubTickerLookup{tickers: map[perp.Contract]*perp.TickerSnapshot{
		"BTC/USDT": {Contract: "BTC/USDT", Last: "50000"},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	proj := projection.NewAccountProjection(ex)
	defer func() {
		cancel()
		proj.Stop()
	}()
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}
	de := NewDecisionEngine(proj, WithContractMetaLookup(testRegistryFromExchange(t, ex)), WithTickerLookup(lookup))
	pct := models.SizingModePercent
	v := "0.1"
	sig := &models.Signal{
		Symbol:     "BTC/USDT",
		Direction:  models.DirectionLong,
		SizingMode: &pct,
		Value:      &v,
		Timestamp:  1,
	}
	order, err := de.ProcessSignal(ctx, "s", sig)
	if err != nil {
		t.Fatal(err)
	}
	if order == nil {
		t.Fatal("expected order")
	}
	if order.Size != "200" {
		t.Fatalf("size: got %q want 200", order.Size)
	}
}
