package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

type purgeRecorder struct {
	logCalls  int
	snapCalls int
}

func (p *purgeRecorder) SaveOrder(context.Context, *models.Order) error { return nil }
func (p *purgeRecorder) ListNonTerminalOrders(context.Context) ([]*models.Order, error) {
	return nil, nil
}
func (p *purgeRecorder) SaveStrategyState(context.Context, string, string, []byte, time.Time) error {
	return nil
}
func (p *purgeRecorder) LoadAllStrategyStates(context.Context) (map[string]map[string][]byte, error) {
	return nil, nil
}
func (p *purgeRecorder) Close() error { return nil }
func (p *purgeRecorder) SaveAccountSnapshot(context.Context, *models.AccountSnapshotRow) error {
	return nil
}
func (p *purgeRecorder) SaveStrategyLog(context.Context, *models.StrategyLogRow) error { return nil }
func (p *purgeRecorder) PurgeAccountSnapshotsBefore(context.Context, time.Time) (int64, error) {
	p.snapCalls++
	return 0, nil
}
func (p *purgeRecorder) PurgeStrategyLogsBefore(context.Context, time.Time) (int64, error) {
	p.logCalls++
	return 0, nil
}

var _ ports.PersistenceStore = (*purgeRecorder)(nil)

func TestRunRetentionOnce_skipsWhenRetentionZero(t *testing.T) {
	rec := &purgeRecorder{}
	_, _, err := runRetentionOnce(context.Background(), rec, config.PersistenceSettings{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec.logCalls != 0 || rec.snapCalls != 0 {
		t.Fatalf("purge called log=%d snap=%d", rec.logCalls, rec.snapCalls)
	}
}

func TestRunRetentionOnce_invokesPurge(t *testing.T) {
	rec := &purgeRecorder{}
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	settings := config.PersistenceSettings{
		StrategyLogRetentionDays:     7,
		AccountSnapshotRetentionDays: 30,
	}
	_, _, err := runRetentionOnce(context.Background(), rec, settings, now)
	if err != nil {
		t.Fatal(err)
	}
	if rec.logCalls != 1 || rec.snapCalls != 1 {
		t.Fatalf("purge calls log=%d snap=%d", rec.logCalls, rec.snapCalls)
	}
}

func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	got := retentionCutoff(now, 7)
	want := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", got, want)
	}
}

func TestRunRetentionOnce_nilStore(t *testing.T) {
	logs, snaps, err := runRetentionOnce(context.Background(), nil, config.DefaultPersistenceSettings(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if logs != 0 || snaps != 0 {
		t.Fatalf("want zero deletes, got logs=%d snaps=%d", logs, snaps)
	}
}
