package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

func (e *Engine) persistAccountSnapshot(balance *perp.BalanceView, positions []*perp.PositionSnapshot, revision uint64, at time.Time) {
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

func encodeAccountSnapshotRow(balance *perp.BalanceView, positions []*perp.PositionSnapshot, revision uint64, at time.Time) (*models.AccountSnapshotRow, error) {
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

	currency := strings.TrimSpace(balance.Currency)
	totalEquity := strings.TrimSpace(balance.Total)

	return &models.AccountSnapshotRow{
		SnapshotAt:    at.UTC(),
		Revision:      revision,
		Currency:      currency,
		TotalEquity:   totalEquity,
		BalanceJSON:   balanceJSON,
		PositionsJSON: positionsJSON,
	}, nil
}
