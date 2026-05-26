package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/kainhuck/signalix/internal/models"
)

func TestOpen_schemaV2(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "v2.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != schemaVersion {
		t.Fatalf("user_version = %d, want %d", v, schemaVersion)
	}
	for _, table := range []string{"account_snapshots", "strategy_logs"} {
		var name string
		err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}

	row := &models.AccountSnapshotRow{
		SnapshotAt:    time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
		Revision:      3,
		Currency:      "USDT",
		TotalEquity:   "1000.5",
		BalanceJSON:   []byte(`{"currency":"USDT","total":"1000.5"}`),
		PositionsJSON: []byte(`[]`),
	}
	if err := s.SaveAccountSnapshot(ctx, row); err != nil {
		t.Fatal(err)
	}

	logRow := &models.StrategyLogRow{
		StrategyName: "trend",
		Level:        "INFO",
		Message:      "hello",
		CreatedAt:    time.Date(2026, 5, 26, 10, 1, 0, 0, time.UTC),
	}
	if err := s.SaveStrategyLog(ctx, logRow); err != nil {
		t.Fatal(err)
	}

	var level string
	if err := s.db.QueryRow(`SELECT level FROM strategy_logs WHERE strategy_name='trend'`).Scan(&level); err != nil {
		t.Fatal(err)
	}
	if level != "info" {
		t.Fatalf("level = %q, want info", level)
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.db")

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE orders (
  id TEXT PRIMARY KEY,
  strategy_name TEXT NOT NULL,
  symbol TEXT NOT NULL,
  side TEXT NOT NULL,
  order_type TEXT NOT NULL,
  size TEXT NOT NULL,
  filled_size TEXT NOT NULL,
  status TEXT NOT NULL,
  price TEXT,
  stop_price TEXT,
  exchange_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE strategy_state (
  strategy_name TEXT NOT NULL,
  state_key TEXT NOT NULL,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (strategy_name, state_key)
);
PRAGMA user_version = 1;
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	s, err := Open(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("user_version = %d, want 2", v)
	}

	ctx := context.Background()
	if err := s.SaveAccountSnapshot(ctx, &models.AccountSnapshotRow{
		SnapshotAt:    time.Now().UTC(),
		Revision:      1,
		Currency:      "USDT",
		TotalEquity:   "1",
		BalanceJSON:   []byte(`{}`),
		PositionsJSON: []byte(`[]`),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeAccountSnapshotsBefore(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "purge_snap.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	times := []time.Time{
		time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	}
	for i, ts := range times {
		if err := s.SaveAccountSnapshot(ctx, &models.AccountSnapshotRow{
			SnapshotAt:    ts,
			Revision:      uint64(i),
			Currency:      "USDT",
			TotalEquity:   fmt.Sprintf("%d", i),
			BalanceJSON:   []byte(`{}`),
			PositionsJSON: []byte(`[]`),
		}); err != nil {
			t.Fatal(err)
		}
	}

	cutoff := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	n, err := s.PurgeAccountSnapshotsBefore(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM account_snapshots`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("remaining rows = %d, want 2", count)
	}
}

func TestPurgeStrategyLogsBefore(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "purge_log.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	for i, ts := range []time.Time{
		time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	} {
		if err := s.SaveStrategyLog(ctx, &models.StrategyLogRow{
			StrategyName: "s1",
			Level:        "info",
			Message:      fmt.Sprintf("m%d", i),
			CreatedAt:    ts,
		}); err != nil {
			t.Fatal(err)
		}
	}

	n, err := s.PurgeStrategyLogsBefore(ctx, time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}
}

func TestSaveStrategyLog_validation(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "val.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.SaveStrategyLog(ctx, &models.StrategyLogRow{StrategyName: "s", Message: ""}); err == nil {
		t.Fatal("expected error for empty message")
	}
}
