package strategy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
)

func TestDiscoverStrategies_defaultMarketPerp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stratDir := filepath.Join(dir, "alpha")
	if err := os.Mkdir(stratDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stratDir, StrategyScriptName), []byte("# stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	const yaml = `
name: alpha
enabled: true
symbols:
  - BTC/USDT
interval: 1m
`
	if err := os.WriteFile(filepath.Join(stratDir, StrategyConfigName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	sl := NewStrategyLoader(t.Context(), dir)
	strategies, err := sl.DiscoverStrategies()
	if err != nil {
		t.Fatal(err)
	}
	st := strategies["alpha"]
	if st == nil || st.Market != models.MarketPerp {
		t.Fatalf("market = %q, want perp", st.Market)
	}
}

func TestDiscoverStrategies_allowsSpotMarket(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stratDir := filepath.Join(dir, "spotty")
	if err := os.Mkdir(stratDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stratDir, StrategyScriptName), []byte("# stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	const yaml = `
name: spotty
enabled: true
market: spot
symbols:
  - BTC/USDT
`
	if err := os.WriteFile(filepath.Join(stratDir, StrategyConfigName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	sl := NewStrategyLoader(t.Context(), dir)
	strategies, err := sl.DiscoverStrategies()
	if err != nil {
		t.Fatal(err)
	}
	if strategies["spotty"].Market != models.MarketSpot {
		t.Fatalf("market = %q", strategies["spotty"].Market)
	}
}

func TestDiscoverStrategies_explicitPerpMarket(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stratDir := filepath.Join(dir, "beta")
	if err := os.Mkdir(stratDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stratDir, StrategyScriptName), []byte("# stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	const yaml = `
name: beta
enabled: true
market: perp
symbols:
  - ETH/USDT
`
	if err := os.WriteFile(filepath.Join(stratDir, StrategyConfigName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	sl := NewStrategyLoader(t.Context(), dir)
	strategies, err := sl.DiscoverStrategies()
	if err != nil {
		t.Fatal(err)
	}
	if strategies["beta"].Market != models.MarketPerp {
		t.Fatalf("market = %q", strategies["beta"].Market)
	}
}
