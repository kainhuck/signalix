package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// NewTestPerpMarkets 构造仅含 perp 的 markets 注册表（测试 / 适配层用）。
func NewTestPerpMarkets(ex ports.Exchange) (map[models.Market]market.Market, error) {
	pm, err := market.NewPerpMarket(context.Background(), market.PerpMarketConfig{Exchange: ex})
	if err != nil {
		return nil, err
	}
	return map[models.Market]market.Market{models.MarketPerp: pm}, nil
}

// testPerpMarkets 构造仅含 perp 的 markets 注册表（测试用）。
func testPerpMarkets(t *testing.T, ex ports.Exchange) map[models.Market]market.Market {
	t.Helper()
	m, err := NewTestPerpMarkets(ex)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
