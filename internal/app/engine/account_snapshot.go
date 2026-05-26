package engine

import (
	"errors"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/projection"
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
	bal, _, _, err := e.accountProjection.Snapshot(perp.Contract("BTC/USDT"))
	if err != nil {
		return nil, err
	}
	return bal, nil
}

// PositionSnapshot 返回指定合约持仓；无仓时 (nil, nil)。
func (e *Engine) PositionSnapshot(contract perp.Contract) (*perp.PositionSnapshot, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return e.accountProjection.PositionForContract(contract)
}

// AllPositionsSnapshot 返回全部持仓快照副本。
func (e *Engine) AllPositionsSnapshot() ([]*perp.PositionSnapshot, error) {
	if err := e.requireProjectionReady(); err != nil {
		return nil, err
	}
	return e.accountProjection.AllPositions()
}
