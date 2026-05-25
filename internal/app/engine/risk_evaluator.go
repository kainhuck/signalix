package engine

import (
	"context"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// StaticRiskEvaluator 基于 risk.Rules 的本地实现。
type StaticRiskEvaluator struct {
	Rules risk.Rules
}

// NewStaticRiskEvaluator 创建静态风控评估器。
func NewStaticRiskEvaluator(rules risk.Rules) *StaticRiskEvaluator {
	return &StaticRiskEvaluator{Rules: rules}
}

// Evaluate 实现 ports.RiskEvaluator。
func (s *StaticRiskEvaluator) Evaluate(ctx context.Context, rc *ports.RiskContext) (*risk.Verdict, error) {
	_ = ctx
	if rc == nil || rc.Order == nil {
		return risk.Allow(), nil
	}
	in := risk.Input{
		Rules:               s.Rules,
		OrderSize:           rc.Order.Size,
		SignalOpensExposure: signalOpensExposure(rc.Signal),
		OpenOrderCount:      rc.OpenOrders,
		OpenPositionCount:   rc.Positions,
	}
	return risk.Evaluate(in), nil
}

func signalOpensExposure(sig *models.Signal) bool {
	if sig == nil {
		return true
	}
	switch sig.Direction {
	case models.DirectionLong, models.DirectionShort:
		return true
	default:
		return false
	}
}
