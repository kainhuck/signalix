package engine

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/shopspring/decimal"
)

func TestStaticRiskEvaluator_Reduce(t *testing.T) {
	rules := risk.DefaultRules()
	rules.MaxOrderSize = decimal.NewFromInt(3)
	ev := NewStaticRiskEvaluator(rules)
	v, err := ev.Evaluate(context.Background(), &ports.RiskContext{
		StrategyName: "s",
		Signal:       &models.Signal{Symbol: "BTC/USDT", Direction: models.DirectionLong},
		Order: &models.Order{
			ID:     "o1",
			Symbol: "BTC/USDT",
			Size:   "10",
		},
		OpenOrders:      0,
		Positions:       0,
		ProjectionReady: true,
		OpensExposure:   true,
		AccountEquity:   decimal.NewFromInt(100000),
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != risk.KindReduce || v.AdjustedSize != "3" {
		t.Fatalf("got %+v", v)
	}
}

func TestStaticRiskEvaluator_NilOrderAllow(t *testing.T) {
	ev := NewStaticRiskEvaluator(risk.DefaultRules())
	v, err := ev.Evaluate(context.Background(), &ports.RiskContext{Order: nil})
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != risk.KindAllow {
		t.Fatalf("got %+v", v)
	}
}
