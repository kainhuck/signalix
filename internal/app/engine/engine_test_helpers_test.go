package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	mktspot "github.com/kainhuck/signalix/internal/app/market/spot"
	apprisk "github.com/kainhuck/signalix/internal/app/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/internal/testutil"
)

func testPerpMarkets(t *testing.T, ex ports.PerpExchange) map[models.Market]market.Market {
	t.Helper()
	pm, err := mktperp.NewPerpMarket(context.Background(), mktperp.PerpMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	return map[models.Market]market.Market{models.MarketPerp: pm}
}

func testSpotMarket(t *testing.T, ex ports.SpotExchange) *mktspot.SpotMarket {
	t.Helper()
	sm, err := mktspot.NewSpotMarket(context.Background(), mktspot.SpotMarketConfig{Exchange: ex})
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.Projection().Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	return sm
}

func testSpotMarkets(t *testing.T, ex ports.SpotExchange) map[models.Market]market.Market {
	t.Helper()
	return map[models.Market]market.Market{models.MarketSpot: testSpotMarket(t, ex)}
}

func testPerpSpotMarkets(t *testing.T, perpEx ports.PerpExchange, spotEx ports.SpotExchange) map[models.Market]market.Market {
	t.Helper()
	markets := testPerpMarkets(t, perpEx)
	markets[models.MarketSpot] = testSpotMarket(t, spotEx)
	return markets
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

func newSpotOnlyTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(t.TempDir(), testSpotMarkets(t, testutil.NewStubSpotExchange()), BuildParams{})
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
