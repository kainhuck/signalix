package oms

import (
	"context"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// testPerpExecutors 为 OMS 测试构造仅含 perp 的 executor map。
func testPerpExecutors(exchange ports.Exchange) (map[models.Market]market.MarketExecutor, *market.PerpExecutor) {
	pe := market.NewPerpExecutor(market.PerpExecutorConfig{Exchange: exchange})
	return map[models.Market]market.MarketExecutor{
		models.MarketPerp: pe,
	}, pe
}

func startTestOMS(ctx context.Context, ee *ExecutionEngine, pe *market.PerpExecutor) error {
	if pe != nil {
		if err := pe.Start(ctx); err != nil {
			return err
		}
	}
	return ee.Start(ctx)
}

func stopTestOMS(ee *ExecutionEngine, pe *market.PerpExecutor) {
	if ee != nil {
		_ = ee.Stop()
	}
	if pe != nil {
		_ = pe.Stop()
	}
}
