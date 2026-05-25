package engine

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

func (e *Engine) setStrategyInterval(name, interval string) {
	e.strategyIntervalMu.Lock()
	defer e.strategyIntervalMu.Unlock()
	if e.strategyIntervals == nil {
		e.strategyIntervals = make(map[string]string)
	}
	e.strategyIntervals[name] = interval
}

func (e *Engine) removeStrategyInterval(name string) {
	e.strategyIntervalMu.Lock()
	defer e.strategyIntervalMu.Unlock()
	delete(e.strategyIntervals, name)
}

func (e *Engine) strategyInterval(name string) (string, bool) {
	e.strategyIntervalMu.RLock()
	defer e.strategyIntervalMu.RUnlock()
	iv, ok := e.strategyIntervals[name]
	return iv, ok
}

func (e *Engine) rpcGetBalance() (interface{}, error) {
	if e.accountProjection == nil {
		return nil, fmt.Errorf("account projection not configured")
	}
	bal, _, _, err := e.accountProjection.Snapshot(perp.Contract("BTC/USDT"))
	if err != nil {
		return nil, err
	}
	return balanceToRPC(bal), nil
}

func (e *Engine) rpcGetPosition(symbol string) (interface{}, error) {
	if e.accountProjection == nil {
		return nil, fmt.Errorf("account projection not configured")
	}
	contract := perp.Contract(strings.TrimSpace(symbol))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	_, pos, _, err := e.accountProjection.Snapshot(contract)
	if err != nil {
		return nil, err
	}
	if pos == nil {
		return nil, nil
	}
	return positionToRPC(pos), nil
}

func (e *Engine) rpcGetKlines(strategyName string, params map[string]interface{}) (interface{}, error) {
	sym, _ := params["symbol"].(string)
	contract := perp.Contract(strings.TrimSpace(sym))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	interval, _ := params["interval"].(string)
	interval = strings.TrimSpace(interval)
	if interval == "" {
		iv, ok := e.strategyInterval(strategyName)
		if !ok || iv == "" {
			return nil, fmt.Errorf("interval is required (strategy not running or no default interval)")
		}
		interval = iv
	}
	limit := 100
	switch v := params["limit"].(type) {
	case float64:
		limit = int(v)
	case int:
		limit = v
	case int64:
		limit = int(v)
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 2000 {
		limit = 2000
	}
	klines := e.router.ListClosedKlines(contract, interval, limit)
	return klines, nil
}

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
