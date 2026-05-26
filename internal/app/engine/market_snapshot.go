package engine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// ErrMarketRouterNotConfigured 表示引擎未配置 MarketRouter。
var ErrMarketRouterNotConfigured = errors.New("market router not configured")

// ErrTickerNotInCache 表示 ticker 不在 MarketRouter 缓存中。
var ErrTickerNotInCache = errors.New("ticker not in cache")

func normalizeKlineLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}

func (e *Engine) requireMarketRouter() error {
	if e == nil || e.router == nil {
		return ErrMarketRouterNotConfigured
	}
	return nil
}

// TickerSnapshot 返回单合约 ticker 缓存快照。
func (e *Engine) TickerSnapshot(contract perp.Contract) (*perp.TickerSnapshot, error) {
	if err := e.requireMarketRouter(); err != nil {
		return nil, err
	}
	contract = perp.Contract(strings.TrimSpace(string(contract)))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	snap, ok := e.router.GetCachedTicker(contract)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTickerNotInCache, contract)
	}
	return snap, nil
}

// ListCachedTickers 返回 ticker 缓存全量副本。
func (e *Engine) ListCachedTickers() (map[perp.Contract]*perp.TickerSnapshot, error) {
	if err := e.requireMarketRouter(); err != nil {
		return nil, err
	}
	return e.router.GetAllCachedTickers(), nil
}

// ClosedKlines 返回最近 limit 根收盘 K 线（时间升序）。
func (e *Engine) ClosedKlines(contract perp.Contract, interval string, limit int) ([]*models.Kline, error) {
	if err := e.requireMarketRouter(); err != nil {
		return nil, err
	}
	contract = perp.Contract(strings.TrimSpace(string(contract)))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return nil, fmt.Errorf("interval is required")
	}
	limit = normalizeKlineLimit(limit)
	klines := e.router.ListClosedKlines(contract, interval, limit)
	if klines == nil {
		return []*models.Kline{}, nil
	}
	return klines, nil
}
