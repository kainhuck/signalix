package engine

import (
	"testing"

	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func TestNewEngine_loadsContractMetaRegistry(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ex.ContractMetas = []*perp.ContractMeta{{
		Contract:         "BTC/USDT",
		QuantoMultiplier: "0.0001",
		OrderSizeMin:     "1",
	}}
	eng := NewEngine(t.TempDir(), testPerpMarkets(t, ex), BuildParams{})
	meta, err := eng.ContractMetaLookup().ContractMeta("BTC/USDT")
	if err != nil {
		t.Fatalf("ContractMeta = %v", err)
	}
	if meta.QuantoMultiplier != "0.0001" {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestEngine_ContractMetaLookup_nilEngine(t *testing.T) {
	t.Parallel()
	var e *Engine
	lookup := e.ContractMetaLookup()
	if lookup == nil {
		t.Fatal("expected non-nil lookup")
	}
	if _, err := lookup.ContractMeta("BTC/USDT"); err == nil {
		t.Fatal("expected error from empty registry")
	}
}
