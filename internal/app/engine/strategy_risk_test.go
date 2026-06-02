package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/app/oms"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/internal/testutil"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

func TestEvaluateStrategyRiskMaxOpenOrders(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	execs, _ := testOMSExecutors(ex)
	ee := oms.NewExecutionEngine(execs, nil)
	existing := &models.Order{
		ID:           "existing",
		Symbol:       "BTC/USDT",
		Side:         models.OrderSideBuy,
		OrderType:    models.OrderTypeLimit,
		Size:         "1",
		Status:       models.OrderStatusSubmitted,
		StrategyName: "s1",
	}
	ee.HydrateFromSnapshot([]*models.Order{existing})

	global := risk.DefaultRules()
	global.MaxOpenOrders = 50
	one := 1
	ov := &risk.Overrides{MaxOpenOrders: &one}

	e := &Engine{
		ctx:             context.Background(),
		executionEngine: ee,
		riskEvaluator:   NewStaticRiskEvaluator(global),
		strategies: map[string]*strategy.Strategy{
			"s1": {
				StrategyConfig: strategy.StrategyConfig{
					Name:    "s1",
					Symbols: []perp.Contract{"BTC/USDT"},
				},
				RiskOverrides: ov,
			},
		},
	}

	order := &models.Order{ID: "o2", Symbol: "BTC/USDT", Size: "1"}
	sig := &models.Signal{Symbol: "BTC/USDT", Direction: models.DirectionLong}
	base := &ports.RiskContext{
		StrategyName:    "s1",
		Signal:          sig,
		Order:           order,
		OpenOrders:      ee.NonFinalOrderCount(),
		Positions:       0,
		ProjectionReady: true,
		OpensExposure:   true,
		AccountEquity:   decimal.NewFromInt(100000),
	}

	v, _, err := e.evaluateStrategyRisk(context.Background(), "s1", sig, order, base)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != risk.KindReject || v.Code != "STRATEGY_RISK_MAX_OPEN_ORDERS" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluateStrategyRiskSkippedWithoutOverrides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := &Engine{
		ctx: ctx,
		strategies: map[string]*strategy.Strategy{
			"s1": {StrategyConfig: strategy.StrategyConfig{Name: "s1"}},
		},
		riskEvaluator: NewStaticRiskEvaluator(risk.DefaultRules()),
	}
	order := &models.Order{ID: "o1", Size: "1"}
	base := &ports.RiskContext{Order: order}
	v, rc, err := e.evaluateStrategyRisk(ctx, "s1", nil, order, base)
	if err != nil || v.Kind != risk.KindAllow || rc != base {
		t.Fatalf("got v=%+v rc=%p base=%p err=%v", v, rc, base, err)
	}
}

func TestOpenPositionCountForStrategy(t *testing.T) {
	t.Parallel()
	ex := testutil.NewStubExchange()
	ex.PositionsList = []*perp.PositionSnapshot{
		{Contract: "BTC/USDT", Size: "1"},
		{Contract: "ETH/USDT", Size: "2"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proj := projection.NewAccountProjection(ex)
	if err := proj.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		proj.Stop()
	}()

	e := &Engine{accountProjection: proj}
	n, err := e.openPositionCountForStrategy([]perp.Contract{"BTC/USDT", "SOL/USDT"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestEvaluateRulesReduceStack(t *testing.T) {
	t.Parallel()
	global := risk.DefaultRules()
	global.MaxOrderSize = decimal.NewFromInt(5)
	strat := risk.MergeRules(global, risk.Overrides{
		MaxOrderSize: decimalPtr(decimal.NewFromInt(3)),
	})
	order := &models.Order{Size: "10"}
	rc := &ports.RiskContext{
		Order:           order,
		Signal:          &models.Signal{Direction: models.DirectionLong},
		ProjectionReady: true,
		OpensExposure:   true,
		AccountEquity:   decimal.NewFromInt(100000),
	}
	v1 := EvaluateRules(strat, rc)
	if v1.AdjustedSize != "3" {
		t.Fatalf("strategy reduce: %+v", v1)
	}
	order.Size = v1.AdjustedSize
	v2 := EvaluateRules(global, rc)
	if v2.Kind != risk.KindAllow {
		t.Fatalf("global after strategy reduce: %+v", v2)
	}
}

func decimalPtr(d decimal.Decimal) *decimal.Decimal { return &d }
