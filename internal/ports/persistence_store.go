package ports

import (
	"context"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

// PersistenceStore 订单/策略状态 + 资产快照与策略日志（sqlite 同一连接）。
type PersistenceStore interface {
	OrderStore

	SaveAccountSnapshot(ctx context.Context, row *models.AccountSnapshotRow) error
	SaveStrategyLog(ctx context.Context, row *models.StrategyLogRow) error

	PurgeAccountSnapshotsBefore(ctx context.Context, before time.Time) (int64, error)
	PurgeStrategyLogsBefore(ctx context.Context, before time.Time) (int64, error)
}
