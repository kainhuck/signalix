package risk

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMergeRulesPartialOverride(t *testing.T) {
	t.Parallel()
	global := DefaultRules()
	global.MaxOpenOrders = 50
	global.MaxOrderSize = decimal.NewFromInt(100000)

	overrides := Overrides{
		MaxOpenOrders: intPtr(10),
	}
	merged := MergeRules(global, overrides)
	if merged.MaxOpenOrders != 10 {
		t.Fatalf("MaxOpenOrders = %d, want 10", merged.MaxOpenOrders)
	}
	if !merged.MaxOrderSize.Equal(decimal.NewFromInt(100000)) {
		t.Fatalf("MaxOrderSize should inherit global, got %s", merged.MaxOrderSize)
	}
}

func TestMergeRulesEnablePositionLock(t *testing.T) {
	t.Parallel()
	global := DefaultRules()
	global.EnablePositionLock = false
	overrides := Overrides{EnablePositionLock: boolPtr(true)}
	merged := MergeRules(global, overrides)
	if !merged.EnablePositionLock {
		t.Fatal("expected position lock true")
	}
}

func TestHasEffectiveOverrides(t *testing.T) {
	t.Parallel()
	if HasEffectiveOverrides(Overrides{}) {
		t.Fatal("empty overrides should not be effective")
	}
	if !HasEffectiveOverrides(Overrides{MaxOpenOrders: intPtr(0)}) {
		t.Fatal("explicit zero should be effective")
	}
	if !HasEffectiveOverrides(Overrides{EnablePositionLock: boolPtr(true)}) {
		t.Fatal("position lock true should be effective")
	}
}

func TestPrefixStrategyVerdict(t *testing.T) {
	t.Parallel()
	allow := PrefixStrategyVerdict(Allow())
	if allow.Code != "RISK_ALLOW" {
		t.Fatalf("allow code: %s", allow.Code)
	}
	rej := PrefixStrategyVerdict(&Verdict{Kind: KindReject, Code: "RISK_MAX_OPEN_ORDERS", Message: "x"})
	if rej.Code != "STRATEGY_RISK_MAX_OPEN_ORDERS" {
		t.Fatalf("code: %s", rej.Code)
	}
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }
