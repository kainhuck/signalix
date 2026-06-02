package engine

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
)

// ErrTickerNotInCache 表示 ticker 不在缓存中（与 market 包同源语义）。
var ErrTickerNotInCache = market.ErrTickerNotInCache

// TickerSnapshot 返回单合约 ticker 缓存快照。
func (e *Engine) TickerSnapshot(symbol string) (*models.Ticker, error) {
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return acct.Ticker(symbol)
}

// ListCachedTickers 返回 ticker 缓存全量副本。
func (e *Engine) ListCachedTickers() (map[string]*models.Ticker, error) {
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	return acct.ListTickers()
}

// ClosedKlines 返回最近 limit 根收盘 K 线（时间升序）。
func (e *Engine) ClosedKlines(symbol string, interval string, limit int) ([]*models.Kline, error) {
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	return acct.Klines(symbol, interval, limit)
}

func (e *Engine) perpAccount() (market.MarketAccount, error) {
	return e.marketAccount(models.MarketPerp)
}
