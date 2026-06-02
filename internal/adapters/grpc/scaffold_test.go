package grpc

import (
	"context"
	"testing"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testEngineWithMarkets(t *testing.T, dir string) *engine.Engine {
	t.Helper()
	pm, err := mktperp.NewPerpMarket(context.Background(), mktperp.PerpMarketConfig{Exchange: testutil.NewStubExchange()})
	if err != nil {
		t.Fatal(err)
	}
	markets := map[models.Market]market.Market{models.MarketPerp: pm}
	build := engine.BuildParams{
		AccountProjection: pm.Projection(),
		MetaLookup:        pm.Registry(),
	}
	return engine.NewEngine(dir, markets, build)
}

func TestCreateStrategy_engineNotRunning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := testEngineWithMarkets(t, dir)
	svc := NewEngineService(e)
	_, err := svc.CreateStrategy(context.Background(), &enginev1.CreateStrategyRequest{
		Name:       "x",
		TemplateId: "trend",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestListTemplates_ok(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := testEngineWithMarkets(t, dir)
	svc := NewEngineService(e)
	reply, err := svc.ListTemplates(context.Background(), &enginev1.ListTemplatesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, tpl := range reply.GetTemplates() {
		ids[tpl.GetId()] = true
	}
	if !ids["trend"] || !ids["blank"] {
		t.Fatalf("templates = %+v", reply.GetTemplates())
	}
}
