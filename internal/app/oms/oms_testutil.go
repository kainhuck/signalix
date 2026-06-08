package oms

import (
	"context"

	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// testPerpExecutors 为 OMS 测试构造仅含 perp 的 executor map。
func testPerpExecutors(exchange ports.PerpExchange) (map[models.Market]market.MarketExecutor, *mktperp.PerpExecutor, error) {
	pe, err := mktperp.NewPerpExecutor(mktperp.PerpExecutorConfig{Exchange: exchange})
	if err != nil {
		return nil, nil, err
	}
	return map[models.Market]market.MarketExecutor{
		models.MarketPerp: pe,
	}, pe, nil
}

func startTestOMS(ctx context.Context, ee *ExecutionEngine, pe *mktperp.PerpExecutor) error {
	if pe != nil {
		if err := pe.Start(ctx); err != nil {
			return err
		}
	}
	return ee.Start(ctx)
}

func stopTestOMS(ee *ExecutionEngine, pe *mktperp.PerpExecutor) {
	if ee != nil {
		_ = ee.Stop()
	}
	if pe != nil {
		_ = pe.Stop()
	}
}
