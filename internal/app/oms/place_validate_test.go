package oms

import (
	"fmt"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func btcMeta() *perp.ContractMeta {
	return &perp.ContractMeta{
		Contract:          "BTC/USDT",
		OrderSizeMin:      "1",
		OrderSizeMax:      "100",
		OrderSizeStep:     "1",
		OrderPriceStep:    "0.5",
		EnableDecimalSize: false,
	}
}

func TestValidateOrderSize_ok(t *testing.T) {
	if err := validateOrderSize(btcMeta(), "10"); err != nil {
		t.Fatalf("validateOrderSize = %v", err)
	}
}

func TestValidateOrderSize_belowMin(t *testing.T) {
	if err := validateOrderSize(btcMeta(), "0.5"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOrderSize_aboveMax(t *testing.T) {
	if err := validateOrderSize(btcMeta(), "101"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOrderSize_notInteger(t *testing.T) {
	if err := validateOrderSize(btcMeta(), "1.5"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOrderSize_stepMisaligned(t *testing.T) {
	meta := btcMeta()
	meta.OrderSizeStep = "2"
	if err := validateOrderSize(meta, "3"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateLimitPrice_tick(t *testing.T) {
	meta := btcMeta()
	if err := validateLimitPrice(meta, "100.3"); err == nil {
		t.Fatal("expected misaligned price error")
	}
	if err := validateLimitPrice(meta, "100.5"); err != nil {
		t.Fatalf("validateLimitPrice = %v", err)
	}
}

func TestValidateOrderForPlace_noLookup(t *testing.T) {
	ee := &ExecutionEngine{}
	order := &models.Order{
		Symbol:    "BTC/USDT",
		Size:      "0",
		OrderType: models.OrderTypeMarket,
	}
	if err := ee.validateOrderForPlace(order); err != nil {
		t.Fatalf("expected skip validation: %v", err)
	}
}

func TestValidateOrderForPlace_nilOrder(t *testing.T) {
	ee := &ExecutionEngine{metaLookup: testMetaRegistryWith(btcMeta())}
	if err := ee.validateOrderForPlace(nil); err == nil {
		t.Fatal("expected error")
	}
}

type testMetaRegistry struct {
	m map[perp.Contract]*perp.ContractMeta
}

func testMetaRegistryWith(metas ...*perp.ContractMeta) *testMetaRegistry {
	m := make(map[perp.Contract]*perp.ContractMeta, len(metas))
	for _, meta := range metas {
		if meta == nil {
			continue
		}
		cp := *meta
		m[meta.Contract] = &cp
	}
	return &testMetaRegistry{m: m}
}

func (r *testMetaRegistry) ContractMeta(contract perp.Contract) (*perp.ContractMeta, error) {
	meta, ok := r.m[contract]
	if !ok || meta == nil {
		return nil, fmt.Errorf("contract meta not found for %s", contract)
	}
	cp := *meta
	return &cp, nil
}
