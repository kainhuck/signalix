package instrument

import (
	"context"
	"errors"
	"testing"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

type stubInstrumentSource struct {
	list []*perp.ContractMeta
	err  error
}

func (s *stubInstrumentSource) ListContractMeta(context.Context) ([]*perp.ContractMeta, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]*perp.ContractMeta, 0, len(s.list))
	for _, m := range s.list {
		if m == nil {
			continue
		}
		cp := *m
		out = append(out, &cp)
	}
	return out, nil
}

func TestRegistry_ContractMeta_empty(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	if _, err := reg.ContractMeta("BTC/USDT"); err == nil {
		t.Fatal("expected error for empty registry")
	}
}

func TestRegistry_LoadFrom_populates(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	src := &stubInstrumentSource{list: []*perp.ContractMeta{{
		Contract:         "BTC/USDT",
		QuantoMultiplier: "0.0001",
		OrderSizeMin:     "1",
	}}}
	if err := reg.LoadFrom(context.Background(), src); err != nil {
		t.Fatalf("LoadFrom = %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("Len = %d", reg.Len())
	}
	meta, err := reg.ContractMeta("BTC/USDT")
	if err != nil {
		t.Fatalf("ContractMeta = %v", err)
	}
	if meta.QuantoMultiplier != "0.0001" || meta.OrderSizeMin != "1" {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestRegistry_ContractMeta_returnsCopy(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	src := &stubInstrumentSource{list: []*perp.ContractMeta{{
		Contract:         "BTC/USDT",
		QuantoMultiplier: "0.0001",
	}}}
	if err := reg.LoadFrom(context.Background(), src); err != nil {
		t.Fatalf("LoadFrom = %v", err)
	}
	meta, err := reg.ContractMeta("BTC/USDT")
	if err != nil {
		t.Fatalf("ContractMeta = %v", err)
	}
	meta.QuantoMultiplier = "changed"
	meta2, err := reg.ContractMeta("BTC/USDT")
	if err != nil {
		t.Fatalf("ContractMeta again = %v", err)
	}
	if meta2.QuantoMultiplier != "0.0001" {
		t.Fatalf("registry mutated: %q", meta2.QuantoMultiplier)
	}
}

func TestRegistry_LoadFrom_replaces(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	if err := reg.LoadFrom(context.Background(), &stubInstrumentSource{list: []*perp.ContractMeta{{
		Contract: "BTC/USDT",
	}}}); err != nil {
		t.Fatalf("first LoadFrom = %v", err)
	}
	if err := reg.LoadFrom(context.Background(), &stubInstrumentSource{list: []*perp.ContractMeta{{
		Contract: "ETH/USDT",
	}}}); err != nil {
		t.Fatalf("second LoadFrom = %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("Len = %d", reg.Len())
	}
	if _, err := reg.ContractMeta("BTC/USDT"); err == nil {
		t.Fatal("expected BTC/USDT removed after replace")
	}
	if _, err := reg.ContractMeta("ETH/USDT"); err != nil {
		t.Fatalf("ETH/USDT = %v", err)
	}
}

func TestRegistry_LoadFrom_skipsInvalid(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	src := &stubInstrumentSource{list: []*perp.ContractMeta{
		nil,
		{Contract: ""},
		{Contract: "BTC/USDT"},
	}}
	if err := reg.LoadFrom(context.Background(), src); err != nil {
		t.Fatalf("LoadFrom = %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("Len = %d", reg.Len())
	}
}

func TestRegistry_LoadFrom_error(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	loadErr := errors.New("list failed")
	if err := reg.LoadFrom(context.Background(), &stubInstrumentSource{err: loadErr}); err == nil {
		t.Fatal("expected LoadFrom error")
	}
	if reg.Len() != 0 {
		t.Fatalf("Len after failed first load = %d", reg.Len())
	}
	if err := reg.LoadFrom(context.Background(), &stubInstrumentSource{list: []*perp.ContractMeta{{
		Contract: "BTC/USDT",
	}}}); err != nil {
		t.Fatalf("successful LoadFrom = %v", err)
	}
	if reg.Len() != 1 {
		t.Fatalf("Len = %d", reg.Len())
	}
	if err := reg.LoadFrom(context.Background(), &stubInstrumentSource{err: loadErr}); err == nil {
		t.Fatal("expected second LoadFrom error")
	}
	if reg.Len() != 1 {
		t.Fatalf("Len after failed reload = %d", reg.Len())
	}
	if _, err := reg.ContractMeta("BTC/USDT"); err != nil {
		t.Fatalf("BTC/USDT should remain: %v", err)
	}
}
