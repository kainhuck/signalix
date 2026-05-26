package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_minimal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ConfigFileName)
	content := `
[log]
log_level = "debug"

[strategies]
dir = "strategies"

[exchange]
api_key = "k"
api_secret = "s"
user_id = "1"

[risk]
enable_risk_control = false
max_order_size = 42
max_open_orders = 3
max_positions = 7
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWorkDir, dir)

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DatabasePath != filepath.Join(dir, DatabaseRelPath) {
		t.Fatalf("db path: %s", got.DatabasePath)
	}
	if got.StrategiesDir != filepath.Join(dir, "strategies") {
		t.Fatalf("strategies: %s", got.StrategiesDir)
	}
	rules := got.RiskRules()
	if rules.Enable || rules.MaxOrderSize.String() != "42" {
		t.Fatalf("risk: %+v", rules)
	}
	opts, err := got.Log.LoggerOptions()
	if err != nil {
		t.Fatal(err)
	}
	_ = opts
}

func TestLoad_missing_config_requires_credentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvWorkDir, dir)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error without exchange credentials")
	}
}

func TestLoad_persistenceDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ConfigFileName)
	content := `
[log]
log_level = "info"

[strategies]
dir = "strategies"

[exchange]
api_key = "k"
api_secret = "s"
user_id = "1"

[risk]
enable_risk_control = false
max_order_size = 1
max_open_orders = 1
max_positions = 1
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWorkDir, dir)

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	ps := got.PersistenceSettings()
	if ps.StrategyLogRetentionDays != 7 {
		t.Fatalf("log retention: %d", ps.StrategyLogRetentionDays)
	}
	if ps.AccountSnapshotRetentionDays != 30 {
		t.Fatalf("snapshot retention: %d", ps.AccountSnapshotRetentionDays)
	}
	if ps.CleanupInterval != time.Hour {
		t.Fatalf("cleanup interval: %s", ps.CleanupInterval)
	}
}

func TestLoad_negativePersistenceRetention(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ConfigFileName)
	content := `
[strategies]
dir = "strategies"

[exchange]
api_key = "k"
api_secret = "s"
user_id = "1"

[risk]
enable_risk_control = false
max_order_size = 1
max_open_orders = 1
max_positions = 1

[persistence]
strategy_log_retention_days = -1
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWorkDir, dir)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for negative retention")
	}
}
