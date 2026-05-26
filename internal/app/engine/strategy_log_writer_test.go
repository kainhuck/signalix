package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

type strategyLogCaptureStore struct {
	mu     sync.Mutex
	rows   []*models.StrategyLogRow
	notify chan struct{}
}

func newStrategyLogCaptureStore() *strategyLogCaptureStore {
	return &strategyLogCaptureStore{notify: make(chan struct{}, 64)}
}

func (s *strategyLogCaptureStore) SaveOrder(context.Context, *models.Order) error { return nil }
func (s *strategyLogCaptureStore) ListNonTerminalOrders(context.Context) ([]*models.Order, error) {
	return nil, nil
}
func (s *strategyLogCaptureStore) SaveStrategyState(context.Context, string, string, []byte, time.Time) error {
	return nil
}
func (s *strategyLogCaptureStore) LoadAllStrategyStates(context.Context) (map[string]map[string][]byte, error) {
	return nil, nil
}
func (s *strategyLogCaptureStore) Close() error { return nil }
func (s *strategyLogCaptureStore) SaveAccountSnapshot(context.Context, *models.AccountSnapshotRow) error {
	return nil
}
func (s *strategyLogCaptureStore) SaveStrategyLog(_ context.Context, row *models.StrategyLogRow) error {
	s.mu.Lock()
	s.rows = append(s.rows, row)
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return nil
}
func (s *strategyLogCaptureStore) PurgeAccountSnapshotsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (s *strategyLogCaptureStore) PurgeStrategyLogsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *strategyLogCaptureStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

var _ ports.PersistenceStore = (*strategyLogCaptureStore)(nil)

func TestEnqueueStrategyLog_persists(t *testing.T) {
	cap := newStrategyLogCaptureStore()
	e := &Engine{
		store: cap,
		ctx:   context.Background(),
	}
	e.startStrategyLogWriter()
	defer e.stopStrategyLogWriter()

	e.enqueueStrategyLog("trend", "INFO", "hello")

	waitForStrategyLog(t, cap, 1)

	cap.mu.Lock()
	row := cap.rows[0]
	cap.mu.Unlock()
	if row.StrategyName != "trend" || row.Message != "hello" {
		t.Fatalf("row: %+v", row)
	}
}

func TestEnqueueStrategyLog_skipsEmptyMessage(t *testing.T) {
	cap := newStrategyLogCaptureStore()
	e := &Engine{store: cap, ctx: context.Background()}
	e.startStrategyLogWriter()
	defer e.stopStrategyLogWriter()

	e.enqueueStrategyLog("trend", "info", "   ")
	if cap.count() != 0 {
		t.Fatalf("expected 0 rows, got %d", cap.count())
	}
}

func TestEnqueueStrategyLog_dropsWhenFull(t *testing.T) {
	cap := newStrategyLogCaptureStore()
	e := &Engine{
		store:         cap,
		ctx:           context.Background(),
		strategyLogCh: make(chan *models.StrategyLogRow, 1),
	}
	// Block worker: channel has capacity 1, pre-fill it.
	e.strategyLogCh <- &models.StrategyLogRow{StrategyName: "blocker", Message: "hold"}

	e.enqueueStrategyLog("trend", "info", "first")
	e.enqueueStrategyLog("trend", "info", "second-should-drop")

	if len(e.strategyLogCh) != 1 {
		t.Fatalf("queue len: %d", len(e.strategyLogCh))
	}
}

func TestStopStrategyLogWriter_drains(t *testing.T) {
	cap := newStrategyLogCaptureStore()
	e := &Engine{
		store: cap,
		ctx:   context.Background(),
	}
	e.startStrategyLogWriter()

	e.enqueueStrategyLog("s1", "info", "a")
	e.enqueueStrategyLog("s1", "info", "b")
	e.stopStrategyLogWriter()

	waitForStrategyLog(t, cap, 2)
}

func waitForStrategyLog(t *testing.T, cap *strategyLogCaptureStore, n int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for cap.count() < n {
		select {
		case <-cap.notify:
		case <-deadline:
			t.Fatalf("timeout waiting for %d logs, got %d", n, cap.count())
		}
	}
}

func TestEnqueueStrategyLog_nilStore(t *testing.T) {
	e := &Engine{ctx: context.Background()}
	e.enqueueStrategyLog("s", "info", "x")
}

func TestEnqueueStrategyLog_nonEmptyWhitespaceMessage(t *testing.T) {
	cap := newStrategyLogCaptureStore()
	e := &Engine{
		store: cap,
		ctx:   context.Background(),
	}
	e.startStrategyLogWriter()
	defer e.stopStrategyLogWriter()

	e.enqueueStrategyLog("s", "info", "  hello  ")
	waitForStrategyLog(t, cap, 1)
	cap.mu.Lock()
	msg := cap.rows[0].Message
	cap.mu.Unlock()
	if msg != "  hello  " {
		t.Fatalf("message stored as %q", msg)
	}
}
