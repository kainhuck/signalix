package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/logger"
)

func strategyLogToProto(row *models.StrategyLogRow) *enginev1.StrategyLog {
	if row == nil {
		return nil
	}
	return &enginev1.StrategyLog{
		Id:              row.ID,
		StrategyName:    row.StrategyName,
		Level:           row.Level,
		Message:         row.Message,
		CreatedAtUnixMs: row.CreatedAt.UnixMilli(),
	}
}

func accountSnapshotToProto(ctx context.Context, row *models.AccountSnapshotRow, includeDetails bool) *enginev1.AccountSnapshot {
	if row == nil {
		return nil
	}
	out := &enginev1.AccountSnapshot{
		Summary: &enginev1.AccountSnapshotSummary{
			Id:               row.ID,
			SnapshotAtUnixMs: row.SnapshotAt.UnixMilli(),
			Revision:         row.Revision,
			Currency:         row.Currency,
			TotalEquity:      row.TotalEquity,
		},
	}
	if !includeDetails {
		return out
	}
	if bal, err := decodePersistedBalanceJSON(row.BalanceJSON); err != nil {
		logger.WarnContext(ctx, "account snapshot balance decode failed",
			logger.Int64("snapshot_id", row.ID),
			logger.Any("error", err))
	} else if bal != nil {
		out.Balance = balanceToProto(bal)
	}
	if positions, err := decodePersistedPositionsJSON(row.PositionsJSON); err != nil {
		logger.WarnContext(ctx, "account snapshot positions decode failed",
			logger.Int64("snapshot_id", row.ID),
			logger.Any("error", err))
	} else {
		for _, p := range positions {
			if p == nil {
				continue
			}
			out.Positions = append(out.Positions, positionToProto(p))
		}
	}
	return out
}

func decodePersistedBalanceJSON(raw []byte) (*models.BalanceView, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty balance json")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	b := &models.BalanceView{
		Currency:  stringField(m, "currency"),
		Total:     stringField(m, "total"),
		Available: stringField(m, "available"),
		Frozen:    stringField(m, "frozen"),
	}
	if ts := int64Field(m, "updated_at"); ts > 0 {
		b.UpdatedAt = time.Unix(ts, 0).UTC()
	}
	return b, nil
}

func decodePersistedPositionsJSON(raw []byte) ([]*models.PositionView, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]*models.PositionView, 0, len(items))
	for _, m := range items {
		if m == nil {
			continue
		}
		lev := ""
		if lv := int64Field(m, "leverage"); lv > 0 {
			lev = strconv.FormatInt(lv, 10)
		}
		p := &models.PositionView{
			Market:        models.MarketPerp,
			Symbol:        stringField(m, "contract"),
			Side:          stringField(m, "side"),
			Size:          stringField(m, "size"),
			EntryPrice:    stringField(m, "entry_price"),
			MarkPrice:     stringField(m, "mark_price"),
			UnrealizedPnl: stringField(m, "unrealized_pnl"),
			Leverage:      lev,
		}
		if ts := int64Field(m, "updated_at"); ts > 0 {
			p.UpdatedAt = time.Unix(ts, 0).UTC()
		}
		out = append(out, p)
	}
	return out, nil
}

func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprint(v)
	}
}

func int64Field(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}
