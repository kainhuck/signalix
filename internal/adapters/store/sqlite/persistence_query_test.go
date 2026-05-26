package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

func TestListStrategyLogs_filterAndOrder(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "list_logs.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	times := []time.Time{
		time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 26, 10, 1, 0, 0, time.UTC),
		time.Date(2026, 5, 26, 10, 2, 0, 0, time.UTC),
	}
	for i, ts := range times {
		row := &models.StrategyLogRow{
			StrategyName: "alpha",
			Level:        "info",
			Message:      "m",
			CreatedAt:    ts,
		}
		if err := s.SaveStrategyLog(ctx, row); err != nil {
			t.Fatal(err)
		}
		if row.ID != int64(i+1) {
			t.Fatalf("id = %d, want %d", row.ID, i+1)
		}
	}
	if err := s.SaveStrategyLog(ctx, &models.StrategyLogRow{
		StrategyName: "beta",
		Level:        "warn",
		Message:      "other",
		CreatedAt:    times[1],
	}); err != nil {
		t.Fatal(err)
	}

	start := times[0]
	end := times[1]
	rows, err := s.ListStrategyLogs(ctx, models.StrategyLogListFilter{
		StrategyName: "alpha",
		StartAt:      &start,
		EndAt:        &end,
		Limit:        10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2", len(rows))
	}
	if rows[0].ID > rows[1].ID {
		t.Fatalf("not ASC: %d then %d", rows[0].ID, rows[1].ID)
	}
}

func TestListRecentStrategyLogs_descLimit(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "recent_logs.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	for i := 0; i < 5; i++ {
		if err := s.SaveStrategyLog(ctx, &models.StrategyLogRow{
			StrategyName: "s1",
			Level:        "info",
			Message:      "m",
			CreatedAt:    time.Date(2026, 5, 26, 10, i, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := s.ListRecentStrategyLogs(ctx, "s1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("len = %d, want 3", len(rows))
	}
	if rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Fatal("expected DESC order")
	}
}

func TestListAccountSnapshots_andLatest(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "list_snaps.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	times := []time.Time{
		time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 26, 10, 1, 0, 0, time.UTC),
	}
	for i, ts := range times {
		if err := s.SaveAccountSnapshot(ctx, &models.AccountSnapshotRow{
			SnapshotAt:    ts,
			Revision:      uint64(i),
			Currency:      "USDT",
			TotalEquity:   "100",
			BalanceJSON:   []byte(`{}`),
			PositionsJSON: []byte(`[]`),
		}); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := s.ListAccountSnapshots(ctx, models.AccountSnapshotListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SnapshotAt.After(rows[1].SnapshotAt) {
		t.Fatalf("rows: %+v", rows)
	}

	latest, err := s.GetLatestAccountSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || !latest.SnapshotAt.Equal(times[1]) {
		t.Fatalf("latest: %+v", latest)
	}
}

func TestGetLatestAccountSnapshot_empty(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "empty_snap.db"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	latest, err := s.GetLatestAccountSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest != nil {
		t.Fatalf("expected nil, got %+v", latest)
	}
}
