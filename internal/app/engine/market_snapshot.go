package engine

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// ErrTickerNotInCache 表示 ticker 不在缓存中（与 market 包同源语义）。
var ErrTickerNotInCache = market.ErrTickerNotInCache

func (e *Engine) perpAccount() (market.MarketAccount, error) {
	return e.marketAccount(models.MarketPerp)
}

// TickerSnapshot 返回单合约 ticker 缓存快照（gRPC shim）。
func (e *Engine) TickerSnapshot(contract perp.Contract) (*perp.TickerSnapshot, error) {
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	contract = perp.Contract(strings.TrimSpace(string(contract)))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	t, err := acct.Ticker(string(contract))
	if err != nil {
		return nil, err
	}
	return tickerModelToPerpSnapshot(t), nil
}

// ListCachedTickers 返回 ticker 缓存全量副本（gRPC shim）。
func (e *Engine) ListCachedTickers() (map[perp.Contract]*perp.TickerSnapshot, error) {
	pm := e.perp()
	if pm == nil || pm.Router() == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	raw := pm.Router().GetAllCachedTickers()
	out := make(map[perp.Contract]*perp.TickerSnapshot, len(raw))
	for k, v := range raw {
		out[k] = v
	}
	return out, nil
}

// ClosedKlines 返回最近 limit 根收盘 K 线（时间升序）。
func (e *Engine) ClosedKlines(contract perp.Contract, interval string, limit int) ([]*models.Kline, error) {
	acct, err := e.perpAccount()
	if err != nil {
		return nil, err
	}
	return acct.Klines(string(contract), interval, limit)
}

func tickerModelToPerpSnapshot(t *models.Ticker) *perp.TickerSnapshot {
	if t == nil {
		return nil
	}
	return &perp.TickerSnapshot{
		Contract:        perp.Contract(t.Contract),
		Last:            t.Last,
		MarkPrice:       t.MarkPrice,
		IndexPrice:      t.IndexPrice,
		FundingRate:     t.FundingRate,
		ChangePct24h:    t.ChangePct24h,
		Volume24h:       t.Volume24h,
		Volume24hBase:   t.Volume24hBase,
		Volume24hQuote:  t.Volume24hQuote,
		OpenInterest:    t.OpenInterest,
		Low24h:          t.Low24h,
		High24h:         t.High24h,
		TimestampMillis: t.TimestampMillis,
	}
}
