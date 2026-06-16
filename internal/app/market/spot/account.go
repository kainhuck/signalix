package spot

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

func (s *SpotMarket) Balance(ctx context.Context, currency string) (*models.BalanceView, error) {
	_ = ctx
	if s == nil || s.proj == nil {
		return nil, fmt.Errorf("spot account projection not configured")
	}
	currency = normalizeCurrency(currency)
	if currency == "" {
		return nil, fmt.Errorf("currency is required")
	}
	return balanceViewToModel(s.proj.Balance(currency)), nil
}

func (s *SpotMarket) Position(ctx context.Context, symbol string) (*models.PositionView, error) {
	_ = ctx
	if s == nil || s.proj == nil {
		return nil, fmt.Errorf("spot account projection not configured")
	}
	pair := spotex.CanonicalPair(symbol)
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	return s.proj.Position(pair), nil
}

func (s *SpotMarket) ListPositions(ctx context.Context) ([]*models.PositionView, error) {
	_ = ctx
	if s == nil || s.proj == nil {
		return nil, fmt.Errorf("spot account projection not configured")
	}
	return s.proj.ListPositions(), nil
}

func (s *SpotMarket) Ticker(symbol string) (*models.Ticker, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	pair := spotex.CanonicalPair(symbol)
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	snap, ok := s.router.GetCachedTicker(pair)
	if !ok {
		return nil, fmt.Errorf("%w: %s", market.ErrTickerNotInCache, pair)
	}
	return tickerFromSpot(snap), nil
}

func (s *SpotMarket) Klines(symbol, interval string, limit int) ([]*models.Kline, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	pair := spotex.CanonicalPair(symbol)
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return nil, fmt.Errorf("interval is required")
	}
	limit = normalizeKlineLimit(limit)
	klines := s.router.ListClosedKlines(pair, interval, limit)
	if klines == nil {
		return []*models.Kline{}, nil
	}
	return klines, nil
}

func (s *SpotMarket) ListTickers() (map[string]*models.Ticker, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	raw := s.router.GetAllCachedTickers()
	out := make(map[string]*models.Ticker, len(raw))
	for pair, snap := range raw {
		if snap == nil {
			continue
		}
		out[pair.String()] = tickerFromSpot(snap)
	}
	return out, nil
}

func normalizeKlineLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > DefaultKlineHistoryMax {
		return DefaultKlineHistoryMax
	}
	return limit
}
