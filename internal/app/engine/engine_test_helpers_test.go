package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	apprisk "github.com/kainhuck/signalix/internal/app/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

func testPerpMarkets(t *testing.T, ex ports.PerpExchange) map[models.Market]market.Market {
	t.Helper()
	pm, err := mktperp.NewPerpMarket(context.Background(), mktperp.PerpMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	return map[models.Market]market.Market{models.MarketPerp: pm}
}

func testPerpSetup(t *testing.T, ex ports.PerpExchange) (map[models.Market]market.Market, BuildParams, *mktperp.PerpMarket) {
	t.Helper()
	pm, err := mktperp.NewPerpMarket(context.Background(), mktperp.PerpMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	markets := map[models.Market]market.Market{models.MarketPerp: pm}
	build := BuildParams{
		AccountProjection: pm.Projection(),
		MetaLookup:        pm.Registry(),
	}
	return markets, build, pm
}

func newTestEngine(t *testing.T, dir string, ex ports.PerpExchange, opts ...EngineOption) *Engine {
	t.Helper()
	markets, build, pm := testPerpSetup(t, ex)
	eng := NewEngine(dir, markets, build, opts...)
	bindTestPerpRisk(eng, pm)
	return eng
}

func bindTestPerpRisk(eng *Engine, pm *mktperp.PerpMarket) {
	if eng == nil || pm == nil {
		return
	}
	eq := apprisk.NewEquityTracker()
	pm.AttachEquityHook(eq.OnEquityUpdate)
	pm.BindRisk(mktperp.PerpRiskConfig{
		Proj:          pm.Projection(),
		Decision:      pm.DecisionEngine(),
		Execution:     eng.ExecutionEngine(),
		Equity:        eq,
		NeedsNotional: eng,
	})
}

func testPerpFromMarkets(t *testing.T, markets map[models.Market]market.Market) *mktperp.PerpMarket {
	t.Helper()
	pm, err := mktperp.PerpFromMarkets(markets)
	if err != nil {
		t.Fatal(err)
	}
	return pm
}
