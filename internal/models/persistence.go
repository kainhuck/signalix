package models

import "time"

// AccountSnapshotRow 资产快照持久化行（EP-2 写入；EP-4 读取）。
type AccountSnapshotRow struct {
	ID            int64
	SnapshotAt    time.Time
	Revision      uint64
	Currency      string
	TotalEquity   string
	BalanceJSON   []byte
	PositionsJSON []byte
}

// StrategyLogRow 策略日志持久化行。
type StrategyLogRow struct {
	ID           int64
	StrategyName string
	Level        string
	Message      string
	CreatedAt    time.Time
}
