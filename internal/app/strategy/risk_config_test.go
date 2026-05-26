package strategy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStrategyConfigWithRisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	const yaml = `
name: test_risk
enabled: true
symbols:
  - BTC/USDT
interval: 1m
risk:
  max_open_orders: 5
  max_daily_loss: 1000
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	sl := NewStrategyLoader(t.Context(), dir)
	cfg, err := sl.LoadStrategyConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Risk == nil || cfg.Risk.MaxOpenOrders == nil || *cfg.Risk.MaxOpenOrders != 5 {
		t.Fatalf("risk parse: %+v", cfg.Risk)
	}
	ov := RiskOverridesFromRaw(cfg.Risk)
	if ov == nil || ov.MaxOpenOrders == nil || *ov.MaxOpenOrders != 5 {
		t.Fatalf("overrides: %+v", ov)
	}
}

func TestLoadStrategyConfigRiskInvalidDrawdown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	const yaml = `
name: bad
enabled: true
symbols:
  - BTC/USDT
risk:
  max_drawdown: 1.5
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	sl := NewStrategyLoader(t.Context(), dir)
	_, err := sl.LoadStrategyConfig(cfgPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRiskOverridesFromRawEmpty(t *testing.T) {
	t.Parallel()
	if RiskOverridesFromRaw(&StrategyRiskConfigRaw{}) != nil {
		t.Fatal("empty risk should yield nil overrides")
	}
}
