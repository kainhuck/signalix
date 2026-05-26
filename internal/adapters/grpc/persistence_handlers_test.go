package grpc

import (
	"context"
	"testing"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type persistenceStubStore struct {
	logs []*models.StrategyLogRow
}

func (p *persistenceStubStore) SaveOrder(context.Context, *models.Order) error { return nil }
func (p *persistenceStubStore) ListNonTerminalOrders(context.Context) ([]*models.Order, error) {
	return nil, nil
}
func (p *persistenceStubStore) SaveStrategyState(context.Context, string, string, []byte, time.Time) error {
	return nil
}
func (p *persistenceStubStore) LoadAllStrategyStates(context.Context) (map[string]map[string][]byte, error) {
	return nil, nil
}
func (p *persistenceStubStore) Close() error { return nil }
func (p *persistenceStubStore) SaveAccountSnapshot(context.Context, *models.AccountSnapshotRow) error {
	return nil
}
func (p *persistenceStubStore) SaveStrategyLog(context.Context, *models.StrategyLogRow) error {
	return nil
}
func (p *persistenceStubStore) PurgeAccountSnapshotsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (p *persistenceStubStore) PurgeStrategyLogsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (p *persistenceStubStore) ListStrategyLogs(_ context.Context, filter models.StrategyLogListFilter) ([]*models.StrategyLogRow, error) {
	return append([]*models.StrategyLogRow(nil), p.logs...), nil
}
func (p *persistenceStubStore) ListRecentStrategyLogs(_ context.Context, _ string, limit int) ([]*models.StrategyLogRow, error) {
	if limit > len(p.logs) {
		limit = len(p.logs)
	}
	if limit <= 0 {
		return nil, nil
	}
	out := append([]*models.StrategyLogRow(nil), p.logs[len(p.logs)-limit:]...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
func (p *persistenceStubStore) ListAccountSnapshots(context.Context, models.AccountSnapshotListFilter) ([]*models.AccountSnapshotRow, error) {
	return nil, nil
}
func (p *persistenceStubStore) GetLatestAccountSnapshot(context.Context) (*models.AccountSnapshotRow, error) {
	return nil, nil
}

func runningEngineWithStore(store *persistenceStubStore) *engine.Engine {
	e := &engine.Engine{}
	engine.WithPersistence(store)(e)
	e.SetRunning(true)
	return e
}

func TestListStrategyLogs_engineNotRunning(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(&engine.Engine{})
	_, err := svc.ListStrategyLogs(context.Background(), &enginev1.ListStrategyLogsRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListStrategyLogs_storeNotConfigured(t *testing.T) {
	t.Parallel()
	e := &engine.Engine{}
	e.SetRunning(true)
	svc := NewEngineService(e)
	_, err := svc.ListStrategyLogs(context.Background(), &enginev1.ListStrategyLogsRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
	if status.Convert(err).Message() != "persistence store not configured" {
		t.Fatalf("msg = %q", status.Convert(err).Message())
	}
}

func TestListStrategyLogs_invalidTimeRange(t *testing.T) {
	t.Parallel()
	svc := NewEngineService(runningEngineWithStore(&persistenceStubStore{}))
	_, err := svc.ListStrategyLogs(context.Background(), &enginev1.ListStrategyLogsRequest{
		StartAtUnixMs: 2000,
		EndAtUnixMs:   1000,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestAccountSnapshotToProto_badJSONKeepsSummary(t *testing.T) {
	row := &models.AccountSnapshotRow{
		ID:            1,
		SnapshotAt:    time.Unix(100, 0).UTC(),
		Revision:      2,
		Currency:      "USDT",
		TotalEquity:   "10",
		BalanceJSON:   []byte(`not-json`),
		PositionsJSON: []byte(`[`),
	}
	out := accountSnapshotToProto(context.Background(), row, true)
	if out.GetSummary().GetTotalEquity() != "10" {
		t.Fatalf("summary missing: %+v", out)
	}
	if out.GetBalance() != nil || len(out.GetPositions()) != 0 {
		t.Fatalf("expected details omitted, got %+v", out)
	}
}

func TestDecodePersistedBalanceJSON_roundTrip(t *testing.T) {
	raw := []byte(`{"currency":"USDT","total":"100","available":"90","frozen":"10","updated_at":1710000000}`)
	bal, err := decodePersistedBalanceJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Currency != "USDT" || bal.Total != "100" {
		t.Fatalf("bal: %+v", bal)
	}
}
