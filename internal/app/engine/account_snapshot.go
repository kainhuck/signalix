package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
)

// ErrAccountProjectionNotConfigured 表示引擎未配置 AccountProjection。
var ErrAccountProjectionNotConfigured = errors.New("account projection not configured")

func (e *Engine) requireProjectionReady() error {
	if e == nil || e.accountProjection == nil {
		return ErrAccountProjectionNotConfigured
	}
	if !e.accountProjection.IsReady() {
		return projection.ErrProjectionNotReady
	}
	return nil
}

// BalanceSnapshot 返回账户余额快照（与 IPC get_balance 同源）。
func (e *Engine) BalanceSnapshot() (*models.BalanceView, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	return acct.Balance(e.ctx, "USDT")
}

// PositionSnapshot 返回指定合约持仓；无仓时 (nil, nil)。
func (e *Engine) PositionSnapshot(symbol string) (*models.PositionView, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return acct.Position(e.ctx, symbol)
}

// AllPositionsSnapshot 返回全部持仓快照副本。
func (e *Engine) AllPositionsSnapshot() ([]*models.PositionView, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	return acct.ListPositions(e.ctx)
}

// balanceToRPC 将余额快照编码为持久化 JSON 载荷（与 IPC 字段一致）。
func balanceToRPC(b *models.BalanceView) map[string]interface{} {
	if b == nil {
		return nil
	}
	return map[string]interface{}{
		"currency":   b.Currency,
		"total":      b.Total,
		"available":  b.Available,
		"frozen":     b.Frozen,
		"updated_at": b.UpdatedAt.Unix(),
	}
}

// positionToRPC 将持仓快照编码为持久化 JSON 载荷。
func positionToRPC(p *models.PositionView) map[string]interface{} {
	if p == nil {
		return nil
	}
	lev := 0
	if p.Leverage != "" {
		lev, _ = strconv.Atoi(strings.TrimSpace(p.Leverage))
	}
	return map[string]interface{}{
		"contract":       p.Symbol,
		"side":           p.Side,
		"size":           p.Size,
		"entry_price":    p.EntryPrice,
		"mark_price":     p.MarkPrice,
		"unrealized_pnl": p.UnrealizedPnl,
		"leverage":       lev,
		"updated_at":     p.UpdatedAt.Unix(),
	}
}
