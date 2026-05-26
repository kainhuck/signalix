package engine

import (
	"context"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

type snapshotCaptureStore struct {
	ch chan *models.AccountSnapshotRow
}

func (s *snapshotCaptureStore) SaveOrder(context.Context, *models.Order) error { return nil }
func (s *snapshotCaptureStore) ListNonTerminalOrders(context.Context) ([]*models.Order, error) {
	return nil, nil
}
func (s *snapshotCaptureStore) SaveStrategyState(context.Context, string, string, []byte, time.Time) error {
	return nil
}
func (s *snapshotCaptureStore) LoadAllStrategyStates(context.Context) (map[string]map[string][]byte, error) {
	return nil, nil
}
func (s *snapshotCaptureStore) Close() error { return nil }
func (s *snapshotCaptureStore) SaveAccountSnapshot(_ context.Context, row *models.AccountSnapshotRow) error {
	s.ch <- row
	return nil
}
func (s *snapshotCaptureStore) SaveStrategyLog(context.Context, *models.StrategyLogRow) error {
	return nil
}
func (s *snapshotCaptureStore) PurgeAccountSnapshotsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (s *snapshotCaptureStore) PurgeStrategyLogsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

var _ ports.PersistenceStore = (*snapshotCaptureStore)(nil)

func TestPersistAccountSnapshot_asyncSave(t *testing.T) {
	cap := &snapshotCaptureStore{ch: make(chan *models.AccountSnapshotRow, 1)}
	e := &Engine{store: cap, ctx: context.Background()}

	at := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	e.persistAccountSnapshot(&perp.BalanceView{
		Currency: "USDT",
		Total:    "100",
	}, nil, 2, at)

	select {
	case row := <-cap.ch:
		if row.TotalEquity != "100" || row.Revision != 2 {
			t.Fatalf("row: %+v", row)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SaveAccountSnapshot")
	}
}

func TestPersistAccountSnapshot_nilStore(t *testing.T) {
	e := &Engine{ctx: context.Background()}
	e.persistAccountSnapshot(&perp.BalanceView{Total: "1"}, nil, 1, time.Now())
}
