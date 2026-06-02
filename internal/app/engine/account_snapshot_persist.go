package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/logger"
)

// PersistAccountSnapshot 持久化账户快照（由 projection refresh 钩子调用）。
func (e *Engine) PersistAccountSnapshot(balance *models.BalanceView, positions []*models.PositionView, revision uint64, at time.Time) {
	if e == nil || e.store == nil {
		return
	}
	row, err := encodeAccountSnapshotRow(balance, positions, revision, at)
	if err != nil {
		logger.WarnContext(e.ctx, "account snapshot encode failed", logger.Any("error", err))
		return
	}
	go func(row *models.AccountSnapshotRow) {
		if err := e.store.SaveAccountSnapshot(context.Background(), row); err != nil {
			logger.WarnContext(e.ctx, "account snapshot persist failed", logger.Any("error", err))
		}
	}(row)
}

func encodeAccountSnapshotRow(balance *models.BalanceView, positions []*models.PositionView, revision uint64, at time.Time) (*models.AccountSnapshotRow, error) {
	if balance == nil {
		return nil, fmt.Errorf("nil balance")
	}
	balanceJSON, err := json.Marshal(balanceToRPC(balance))
	if err != nil {
		return nil, err
	}

	posPayload := make([]map[string]interface{}, 0, len(positions))
	for _, pv := range positions {
		if pv == nil {
			continue
		}
		posPayload = append(posPayload, positionToRPC(pv))
	}
	positionsJSON, err := json.Marshal(posPayload)
	if err != nil {
		return nil, err
	}

	return &models.AccountSnapshotRow{
		SnapshotAt:    at,
		Revision:      revision,
		Currency:      balance.Currency,
		TotalEquity:   balance.Total,
		BalanceJSON:   balanceJSON,
		PositionsJSON: positionsJSON,
	}, nil
}
