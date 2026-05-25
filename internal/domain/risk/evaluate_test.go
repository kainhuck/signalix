package risk

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestEvaluate_DisabledAlwaysAllow(t *testing.T) {
	in := Input{
		Rules: Rules{Enable: false},
	}
	v := Evaluate(in)
	if v.Kind != KindAllow {
		t.Fatalf("kind: %v", v.Kind)
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
