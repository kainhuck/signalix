package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
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
func (e *Engine) BalanceSnapshot() (*perp.BalanceView, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	bv, err := acct.Balance(e.ctx, "USDT")
	if err != nil {
		return nil, err
	}
	return balanceModelToPerp(bv), nil
}

// PositionSnapshot 返回指定合约持仓；无仓时 (nil, nil)。
func (e *Engine) PositionSnapshot(contract perp.Contract) (*perp.PositionSnapshot, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	pv, err := acct.Position(e.ctx, string(contract))
	if err != nil {
		return nil, err
	}
	return positionModelToPerp(pv), nil
}

// AllPositionsSnapshot 返回全部持仓快照副本。
func (e *Engine) AllPositionsSnapshot() ([]*perp.PositionSnapshot, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	return e.accountProjection.AllPositions()
}

func balanceModelToPerp(b *models.BalanceView) *perp.BalanceView {
	if b == nil {
		return nil
	}
	return &perp.BalanceView{
		Currency:  b.Currency,
		Total:     b.Total,
		Available: b.Available,
		Frozen:    b.Frozen,
		UpdatedAt: b.UpdatedAt,
	}
}

func positionModelToPerp(p *models.PositionView) *perp.PositionSnapshot {
	if p == nil {
		return nil
	}
	return &perp.PositionSnapshot{
		Contract:      perp.Contract(p.Symbol),
		Side:          perp.PositionSide(p.Side),
		Size:          p.Size,
		EntryPrice:    p.EntryPrice,
		MarkPrice:     p.MarkPrice,
		UnrealizedPnl: p.UnrealizedPnl,
		Leverage:      parseLeverageInt(p.Leverage),
		UpdatedAt:     p.UpdatedAt,
	}
}

func parseLeverageInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// balanceToRPC 将 perp 余额快照编码为持久化 JSON 载荷（与 IPC 字段一致）。
func balanceToRPC(b *perp.BalanceView) map[string]interface{} {
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

// positionToRPC 将 perp 持仓快照编码为持久化 JSON 载荷。
func positionToRPC(p *perp.PositionSnapshot) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"contract":       string(p.Contract),
		"side":           string(p.Side),
		"size":           p.Size,
		"entry_price":    p.EntryPrice,
		"mark_price":     p.MarkPrice,
		"unrealized_pnl": p.UnrealizedPnl,
		"leverage":       p.Leverage,
		"updated_at":     p.UpdatedAt.Unix(),
	}
}
