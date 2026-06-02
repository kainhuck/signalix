package scaffold_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/app/strategy/scaffold"
	"gopkg.in/yaml.v3"
)

func TestValidateName(t *testing.T) {
	t.Parallel()
	if err := scaffold.ValidateName("my_trend-1"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := scaffold.ValidateName("Bad"); err == nil {
		t.Fatal("want invalid")
	}
	if err := scaffold.ValidateName(""); err == nil {
		t.Fatal("want invalid")
	}
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if err := scaffold.ValidateName(string(long)); err == nil {
		t.Fatal("want invalid for long name")
	}
}

func TestListTemplates(t *testing.T) {
	t.Parallel()
	list, err := scaffold.ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 {
		t.Fatalf("templates = %+v", list)
	}
	seen := map[string]bool{}
	for _, m := range list {
		seen[m.ID] = true
	}
	if !seen["trend"] || !seen["blank"] {
		t.Fatalf("missing template: %+v", seen)
	}
}

func TestCreate_trend_defaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: dir,
		Name:          "alpha",
		TemplateID:    "trend",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "alpha") {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(filepath.Join(dir, "alpha", strategy.StrategyConfigName))
	if err != nil {
		t.Fatal(err)
	}
	var cfg strategy.StrategyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "alpha" || cfg.Enabled {
		t.Fatalf("cfg = %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha", strategy.StrategyScriptName)); err != nil {
		t.Fatal(err)
	}
}

func TestCreate_mergeSymbols(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: dir,
		Name:          "s1",
		TemplateID:    "trend",
		Symbols:       []string{"ETH/USDT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "s1", strategy.StrategyConfigName))
	var cfg strategy.StrategyConfig
	_ = yaml.Unmarshal(data, &cfg)
	if len(cfg.Symbols) != 1 || cfg.Symbols[0] != "ETH/USDT" {
		t.Fatalf("symbols = %+v", cfg.Symbols)
	}
}

func TestCreate_mergeParameters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: dir,
		Name:          "s2",
		TemplateID:    "trend",
		Parameters:    map[string]interface{}{"ma_fast": float64(5)},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "s2", strategy.StrategyConfigName))
	var cfg strategy.StrategyConfig
	_ = yaml.Unmarshal(data, &cfg)
	if cfg.Parameters["ma_fast"] != 5 {
		t.Fatalf("params = %+v", cfg.Parameters)
	}
	if cfg.Parameters["ma_slow"] == nil {
		t.Fatal("expected template default ma_slow")
	}
}

func TestCreate_enabledExplicit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	enabled := true
	_, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: dir,
		Name:          "s3",
		TemplateID:    "trend",
		Enabled:       &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "s3", strategy.StrategyConfigName))
	var cfg strategy.StrategyConfig
	_ = yaml.Unmarshal(data, &cfg)
	if !cfg.Enabled {
		t.Fatal("expected enabled true")
	}
}

func TestCreate_alreadyExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := scaffold.CreateOptions{StrategiesDir: dir, Name: "dup", TemplateID: "blank"}
	if _, err := scaffold.Create(opts); err != nil {
		t.Fatal(err)
	}
	_, err := scaffold.Create(opts)
	if !errors.Is(err, scaffold.ErrAlreadyExists) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreate_unknownTemplate(t *testing.T) {
	t.Parallel()
	_, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: t.TempDir(),
		Name:          "x",
		TemplateID:    "missing",
	})
	if !errors.Is(err, scaffold.ErrTemplateNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreate_invalidInterval(t *testing.T) {
	t.Parallel()
	bad := "2w"
	_, err := scaffold.Create(scaffold.CreateOptions{
		StrategiesDir: t.TempDir(),
		Name:          "bad_iv",
		TemplateID:    "trend",
		Interval:      &bad,
	})
	if !errors.Is(err, scaffold.ErrInvalidConfig) {
		t.Fatalf("err = %v", err)
	}
}
