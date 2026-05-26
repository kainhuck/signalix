package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

func formatTimeUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// SaveAccountSnapshot INSERT 一行资产快照。
func (s *Store) SaveAccountSnapshot(ctx context.Context, row *models.AccountSnapshotRow) error {
	if row == nil {
		return fmt.Errorf("nil account snapshot row")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO account_snapshots (snapshot_at, revision, currency, total_equity, balance_json, positions_json)
VALUES (?, ?, ?, ?, ?, ?)
`,
		formatTimeUTC(row.SnapshotAt),
		row.Revision,
		row.Currency,
		row.TotalEquity,
		string(row.BalanceJSON),
		string(row.PositionsJSON),
	)
	return err
}

// SaveStrategyLog INSERT 一行策略日志。
func (s *Store) SaveStrategyLog(ctx context.Context, row *models.StrategyLogRow) error {
	if row == nil {
		return fmt.Errorf("nil strategy log row")
	}
	if strings.TrimSpace(row.StrategyName) == "" {
		return fmt.Errorf("empty strategy_name")
	}
	if strings.TrimSpace(row.Message) == "" {
		return fmt.Errorf("empty message")
	}
	level := strings.ToLower(strings.TrimSpace(row.Level))
	if level == "" {
		level = "info"
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO strategy_logs (strategy_name, level, message, created_at)
VALUES (?, ?, ?, ?)
`,
		row.StrategyName,
		level,
		row.Message,
		formatTimeUTC(row.CreatedAt),
	)
	return err
}

// PurgeAccountSnapshotsBefore 删除 snapshot_at 严格早于 before 的行。
func (s *Store) PurgeAccountSnapshotsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM account_snapshots WHERE snapshot_at < ?
`, formatTimeUTC(before))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeStrategyLogsBefore 删除 created_at 严格早于 before 的行。
func (s *Store) PurgeStrategyLogsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM strategy_logs WHERE created_at < ?
`, formatTimeUTC(before))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
