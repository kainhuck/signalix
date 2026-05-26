package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

func parseTimeUTC(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func scanStrategyLogRow(scanner interface {
	Scan(dest ...any) error
}) (*models.StrategyLogRow, error) {
	var (
		id           int64
		strategyName string
		level        string
		message      string
		createdS     string
	)
	if err := scanner.Scan(&id, &strategyName, &level, &message, &createdS); err != nil {
		return nil, err
	}
	createdAt, err := parseTimeUTC(createdS)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	return &models.StrategyLogRow{
		ID:           id,
		StrategyName: strategyName,
		Level:        level,
		Message:      message,
		CreatedAt:    createdAt,
	}, nil
}

func scanAccountSnapshotRow(scanner interface {
	Scan(dest ...any) error
}) (*models.AccountSnapshotRow, error) {
	var (
		id            int64
		snapshotS     string
		revision      uint64
		currency      string
		totalEquity   string
		balanceJSON   string
		positionsJSON string
	)
	if err := scanner.Scan(&id, &snapshotS, &revision, &currency, &totalEquity, &balanceJSON, &positionsJSON); err != nil {
		return nil, err
	}
	snapshotAt, err := parseTimeUTC(snapshotS)
	if err != nil {
		return nil, fmt.Errorf("parse snapshot_at: %w", err)
	}
	return &models.AccountSnapshotRow{
		ID:            id,
		SnapshotAt:    snapshotAt,
		Revision:      revision,
		Currency:      currency,
		TotalEquity:   totalEquity,
		BalanceJSON:   []byte(balanceJSON),
		PositionsJSON: []byte(positionsJSON),
	}, nil
}

const accountSnapshotSelectCols = `
id, snapshot_at, revision, currency, total_equity, balance_json, positions_json
`

// ListStrategyLogs 按 filter 查询策略日志（created_at ASC, id ASC）。
func (s *Store) ListStrategyLogs(ctx context.Context, filter models.StrategyLogListFilter) ([]*models.StrategyLogRow, error) {
	var (
		args  []any
		where []string
	)
	if name := strings.TrimSpace(filter.StrategyName); name != "" {
		where = append(where, "strategy_name = ?")
		args = append(args, name)
	}
	if filter.StartAt != nil {
		where = append(where, "created_at >= ?")
		args = append(args, formatTimeUTC(*filter.StartAt))
	}
	if filter.EndAt != nil {
		where = append(where, "created_at <= ?")
		args = append(args, formatTimeUTC(*filter.EndAt))
	}
	query := `
SELECT id, strategy_name, level, message, created_at
FROM strategy_logs
`
	if len(where) > 0 {
		query += "WHERE " + strings.Join(where, " AND ") + "\n"
	}
	query += "ORDER BY created_at ASC, id ASC\n"
	if filter.Limit > 0 {
		query += "LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.StrategyLogRow
	for rows.Next() {
		row, err := scanStrategyLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListRecentStrategyLogs 按 created_at DESC, id DESC 取最近 limit 条。
func (s *Store) ListRecentStrategyLogs(ctx context.Context, strategyName string, limit int) ([]*models.StrategyLogRow, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		args  []any
		where string
	)
	if name := strings.TrimSpace(strategyName); name != "" {
		where = "WHERE strategy_name = ?"
		args = append(args, name)
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
SELECT id, strategy_name, level, message, created_at
FROM strategy_logs
%s
ORDER BY created_at DESC, id DESC
LIMIT ?
`, where)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.StrategyLogRow
	for rows.Next() {
		row, err := scanStrategyLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListAccountSnapshots 按 snapshot_at ASC, id ASC 查询资产快照。
func (s *Store) ListAccountSnapshots(ctx context.Context, filter models.AccountSnapshotListFilter) ([]*models.AccountSnapshotRow, error) {
	var (
		args  []any
		where []string
	)
	if filter.StartAt != nil {
		where = append(where, "snapshot_at >= ?")
		args = append(args, formatTimeUTC(*filter.StartAt))
	}
	if filter.EndAt != nil {
		where = append(where, "snapshot_at <= ?")
		args = append(args, formatTimeUTC(*filter.EndAt))
	}
	query := "SELECT " + accountSnapshotSelectCols + " FROM account_snapshots\n"
	if len(where) > 0 {
		query += "WHERE " + strings.Join(where, " AND ") + "\n"
	}
	query += "ORDER BY snapshot_at ASC, id ASC\n"
	if filter.Limit > 0 {
		query += "LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AccountSnapshotRow
	for rows.Next() {
		row, err := scanAccountSnapshotRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetLatestAccountSnapshot 返回 snapshot_at 最大的一行。
func (s *Store) GetLatestAccountSnapshot(ctx context.Context) (*models.AccountSnapshotRow, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+accountSnapshotSelectCols+`
FROM account_snapshots
ORDER BY snapshot_at DESC, id DESC
LIMIT 1
`)
	out, err := scanAccountSnapshotRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return out, err
}
