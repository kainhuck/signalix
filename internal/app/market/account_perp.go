package market

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

const perpBalanceAnchorContract = "BTC/USDT"

// Balance 满足 MarketAccount（perp）。
func (p *PerpMarket) Balance(ctx context.Context, currency string) (*models.BalanceView, error) {
	_ = ctx
	_ = currency
	if p == nil || p.proj == nil {
		return nil, fmt.Errorf("account projection not configured")
	}
	bal, _, _, err := p.proj.Snapshot(perp.Contract(perpBalanceAnchorContract))
	if err != nil {
		return nil, err
	}
	return balanceViewFromPerp(bal), nil
}

// Position 满足 MarketAccount（perp）。
func (p *PerpMarket) Position(ctx context.Context, symbol string) (*models.PositionView, error) {
	_ = ctx
	if p == nil || p.proj == nil {
		return nil, fmt.Errorf("account projection not configured")
	}
	contract := perp.Contract(strings.TrimSpace(symbol))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	_, pos, _, err := p.proj.Snapshot(contract)
	if err != nil {
		return nil, err
	}
	if pos == nil {
		return nil, nil
	}
	return positionViewFromPerp(pos), nil
}

// Ticker 满足 MarketAccount（perp）。
func (p *PerpMarket) Ticker(symbol string) (*models.Ticker, error) {
	if p == nil || p.router == nil {
		return nil, ErrMarketRouterNotConfigured
	}
	contract := perp.Contract(strings.TrimSpace(symbol))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	snap, ok := p.router.GetCachedTicker(contract)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTickerNotInCache, contract)
	}
	return models.TickerFromSnapshot(snap), nil
}

// Klines 满足 MarketAccount（perp）。
func (p *PerpMarket) Klines(symbol, interval string, limit int) ([]*models.Kline, error) {
	if p == nil || p.router == nil {
		return nil, ErrMarketRouterNotConfigured
	}
	contract := perp.Contract(strings.TrimSpace(symbol))
	if contract == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return nil, fmt.Errorf("interval is required")
	}
	limit = normalizeKlineLimit(limit)
	klines := p.router.ListClosedKlines(contract, interval, limit)
	if klines == nil {
		return []*models.Kline{}, nil
	}
	return klines, nil
}

func balanceViewFromPerp(b *perp.BalanceView) *models.BalanceView {
	if b == nil {
		return nil
	}
	return &models.BalanceView{
		Currency:  b.Currency,
		Total:     b.Total,
		Available: b.Available,
		Frozen:    b.Frozen,
		UpdatedAt: b.UpdatedAt,
	}
}

func positionViewFromPerp(pos *perp.PositionSnapshot) *models.PositionView {
	if pos == nil {
		return nil
	}
	lev := ""
	if pos.Leverage != 0 {
		lev = strconv.Itoa(pos.Leverage)
	}
	return &models.PositionView{
		Market:        models.MarketPerp,
		Symbol:        string(pos.Contract),
		Side:          string(pos.Side),
		Size:          pos.Size,
		EntryPrice:    pos.EntryPrice,
		MarkPrice:     pos.MarkPrice,
		UnrealizedPnl: pos.UnrealizedPnl,
		Leverage:      lev,
		UpdatedAt:     pos.UpdatedAt,
	}
}

func normalizeKlineLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}
