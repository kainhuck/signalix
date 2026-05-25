package risk

import (
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

func TestEvaluate_DisabledAlwaysAllow(t *testing.T) {
	in := Input{
		Rules: Rules{Enable: false},
	}
	v := Evaluate(in)
	if v.Kind != KindAllow {
		t.Fatalf("kind: %v", v)
	}
}

func TestEvaluate_InvalidSize(t *testing.T) {
	in := Input{
		Rules:     DefaultRules(),
		OrderSize: "not-a-number",
	}
	v := Evaluate(in)
	if v.Kind != KindReject || v.Code != "RISK_INVALID_ORDER_SIZE" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxOpenOrders(t *testing.T) {
	r := DefaultRules()
	r.MaxOpenOrders = 2
	in := Input{
		Rules:          r,
		OrderSize:      "1",
		OpenOrderCount: 2,
	}
	v := Evaluate(in)
	if v.Kind != KindReject || v.Code != "RISK_MAX_OPEN_ORDERS" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxPositions_SkipWhenUnknown(t *testing.T) {
	r := DefaultRules()
	r.MaxPositions = 1
	in := Input{
		Rules:               r,
		OrderSize:           "1",
		SignalOpensExposure: true,
		OpenPositionCount:   -1,
	}
	v := Evaluate(in)
	if v.Kind != KindAllow {
		t.Fatalf("expected allow when position count unknown, got %+v", v)
	}
}

func TestEvaluate_MaxPositions_Reject(t *testing.T) {
	r := DefaultRules()
	r.MaxPositions = 2
	in := Input{
		Rules:               r,
		OrderSize:           "1",
		SignalOpensExposure: true,
		OpenPositionCount:   2,
	}
	v := Evaluate(in)
	if v.Kind != KindReject || v.Code != "RISK_MAX_POSITIONS" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxPositions_FlatIgnored(t *testing.T) {
	r := DefaultRules()
	r.MaxPositions = 1
	in := Input{
		Rules:               r,
		OrderSize:           "1",
		SignalOpensExposure: false,
		OpenPositionCount:   99,
	}
	v := Evaluate(in)
	if v.Kind != KindAllow {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_ReduceOrderSize(t *testing.T) {
	r := DefaultRules()
	r.MaxOrderSize = decimal.NewFromInt(10)
	in := Input{
		Rules:          r,
		OrderSize:      "25",
		OpenOrderCount: 0,
	}
	v := Evaluate(in)
	if v.Kind != KindReduce || v.AdjustedSize != "10" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_ProjectionNotReady(t *testing.T) {
	r := DefaultRules()
	in := Input{
		Rules:           r,
		OrderSize:       "1",
		OpensExposure:   true,
		ProjectionReady: false,
	}
	v := Evaluate(in)
	if v.Code != "RISK_PROJECTION_NOT_READY" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_ProjectionNotReadyFlatAllowed(t *testing.T) {
	r := DefaultRules()
	in := Input{
		Rules:           r,
		OrderSize:       "1",
		OpensExposure:   false,
		ProjectionReady: false,
	}
	v := Evaluate(in)
	if v.Kind != KindAllow {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_PositionLock(t *testing.T) {
	r := DefaultRules()
	r.EnablePositionLock = true
	in := Input{
		Rules:             r,
		OrderSize:         "1",
		OpensExposure:     true,
		ProjectionReady:   true,
		AccountEquityUSDT: decimal.NewFromInt(1000),
	}
	v := Evaluate(in)
	if v.Code != "RISK_POSITION_LOCK" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxDailyLoss(t *testing.T) {
	r := DefaultRules()
	r.MaxDailyLoss = decimal.NewFromInt(500)
	in := Input{
		Rules:             r,
		OrderSize:         "1",
		OpensExposure:     true,
		ProjectionReady:   true,
		AccountEquityUSDT: decimal.NewFromInt(1000),
		DailyLossUSDT:     decimal.NewFromInt(500),
	}
	v := Evaluate(in)
	if v.Code != "RISK_MAX_DAILY_LOSS" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxDrawdown(t *testing.T) {
	r := DefaultRules()
	r.MaxDrawdown = decimal.RequireFromString("0.2")
	in := Input{
		Rules:             r,
		OrderSize:         "1",
		OpensExposure:     true,
		ProjectionReady:   true,
		AccountEquityUSDT: decimal.NewFromInt(1000),
		DrawdownRatio:     decimal.RequireFromString("0.25"),
	}
	v := Evaluate(in)
	if v.Code != "RISK_MAX_DRAWDOWN" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxLeverage(t *testing.T) {
	r := DefaultRules()
	r.MaxLeverage = 5
	in := Input{
		Rules:             r,
		OrderSize:         "1",
		OpensExposure:     true,
		ProjectionReady:   true,
		NotionalAvailable: true,
		AccountEquityUSDT: decimal.NewFromInt(1000),
		PostLeverage:      decimal.NewFromInt(6),
	}
	v := Evaluate(in)
	if v.Code != "RISK_MAX_LEVERAGE" {
		t.Fatalf("got %+v", v)
	}
}

func TestEvaluate_MaxPositionSizeReduce(t *testing.T) {
	r := DefaultRules()
	r.MaxPositionSize = decimal.NewFromInt(1000)
	in := Input{
		Rules:                    r,
		OrderSize:                "10",
		IncreasingExposure:       true,
		NotionalAvailable:        true,
		ProjectionReady:          true,
		AccountEquityUSDT:        decimal.NewFromInt(10000),
		PositionNotionalUSDT:     decimal.NewFromInt(500),
		PostPositionNotionalUSDT: decimal.NewFromInt(1500),
		NotionalUSDTPerContract:  decimal.NewFromInt(100),
	}
	v := Evaluate(in)
	if v.Kind != KindReduce || v.AdjustedSize != "5" {
		t.Fatalf("got %+v", v)
	}
}

func TestOpensExposure(t *testing.T) {
	if !OpensExposure(&models.Signal{Direction: models.DirectionLong}) {
		t.Fatal("long should open")
	}
	if OpensExposure(&models.Signal{Direction: models.DirectionFlat}) {
		t.Fatal("flat should not open")
	}
}

func TestIsIncreasingExposure(t *testing.T) {
	if !IsIncreasingExposure(&models.Signal{Direction: models.DirectionLong}, nil) {
		t.Fatal("long from flat")
	}
	pos := &perp.PositionSnapshot{Side: perp.PositionLong, Size: "1"}
	if !IsIncreasingExposure(&models.Signal{Direction: models.DirectionLong}, pos) {
		t.Fatal("long add to long")
	}
	if IsIncreasingExposure(&models.Signal{Direction: models.DirectionFlat}, pos) {
		t.Fatal("flat not increasing")
	}
}
