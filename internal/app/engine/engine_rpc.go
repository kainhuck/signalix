package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
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

func (e *Engine) marketAccount(m models.Market) (market.MarketAccount, error) {
	if e == nil {
		return nil, fmt.Errorf("engine not configured")
	}
	mk, ok := e.markets[m]
	if !ok || mk == nil {
		return nil, fmt.Errorf("market %q not registered", m)
	}
	return mk, nil
}

func (e *Engine) rpcGetBalance() (interface{}, error) {
	acct, err := e.marketAccount(models.MarketPerp)
	if err != nil {
		return nil, err
	}
	bv, err := acct.Balance(context.Background(), "USDT")
	if err != nil {
		return nil, err
	}
	return balanceViewToRPC(bv), nil
}

func (e *Engine) rpcGetPosition(symbol string) (interface{}, error) {
	acct, err := e.marketAccount(models.MarketPerp)
	if err != nil {
		return nil, err
	}
	pv, err := acct.Position(context.Background(), symbol)
	if err != nil {
		return nil, err
	}
	if pv == nil {
		return nil, nil
	}
	return positionViewToRPC(pv), nil
}

func (e *Engine) rpcGetKlines(strategyName string, params map[string]interface{}) (interface{}, error) {
	acct, err := e.marketAccount(models.MarketPerp)
	if err != nil {
		return nil, err
	}
	sym, _ := params["symbol"].(string)
	sym = strings.TrimSpace(sym)
	if sym == "" {
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
	return acct.Klines(sym, interval, limit)
}

func (e *Engine) rpcGetTicker(symbol string) (interface{}, error) {
	acct, err := e.marketAccount(models.MarketPerp)
	if err != nil {
		return nil, err
	}
	t, err := acct.Ticker(strings.TrimSpace(symbol))
	if err != nil {
		return nil, err
	}
	return tickerViewToRPC(t), nil
}

func tickerViewToRPC(t *models.Ticker) map[string]interface{} {
	if t == nil {
		return nil
	}
	return map[string]interface{}{
		"contract":         string(t.Contract),
		"last":             t.Last,
		"mark_price":       t.MarkPrice,
		"index_price":      t.IndexPrice,
		"funding_rate":     t.FundingRate,
		"change_pct_24h":   t.ChangePct24h,
		"volume_24h":       t.Volume24h,
		"volume_24h_base":  t.Volume24hBase,
		"volume_24h_quote": t.Volume24hQuote,
		"open_interest":    t.OpenInterest,
		"low_24h":          t.Low24h,
		"high_24h":         t.High24h,
		"timestamp_millis": t.TimestampMillis,
	}
}

func balanceViewToRPC(b *models.BalanceView) map[string]interface{} {
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

func positionViewToRPC(p *models.PositionView) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"contract":       p.Symbol,
		"side":           p.Side,
		"size":           p.Size,
		"entry_price":    p.EntryPrice,
		"mark_price":     p.MarkPrice,
		"unrealized_pnl": p.UnrealizedPnl,
		"leverage":       p.Leverage,
		"updated_at":     p.UpdatedAt.Unix(),
	}
}
