package engine

import (
	"context"

	"github.com/kainhuck/signalix/internal/domain/risk"
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

// EvaluateRules 使用指定规则评估 RiskContext（纯映射 + domain Evaluate）。
func EvaluateRules(rules risk.Rules, rc *ports.RiskContext) *risk.Verdict {
	if rc == nil || rc.Order == nil {
		return risk.Allow()
	}
	in := risk.Input{
		Rules:                rules,
		OrderSize:            rc.Order.Size,
		SignalOpensExposure:  risk.OpensExposure(rc.Signal),
		OpenOrderCount:       rc.OpenOrders,
		OpenPositionCount:    rc.Positions,
		ProjectionReady:      rc.ProjectionReady,
		NotionalAvailable:    rc.NotionalAvailable,
		OpensExposure:        rc.OpensExposure,
		IncreasingExposure:   rc.IncreasingExposure,
		OrderNotional:        rc.OrderNotional,
		PositionNotional:     rc.PositionNotional,
		PostPositionNotional: rc.PostPositionNotional,
		TotalExposure:        rc.TotalExposure,
		PostTotalExposure:    rc.PostTotalExposure,
		AccountEquity:        rc.AccountEquity,
		DailyLoss:            rc.DailyLoss,
		DrawdownRatio:        rc.DrawdownRatio,
		PostLeverage:         rc.PostLeverage,
		NotionalPerUnit:      rc.NotionalPerUnit,
	}
	return risk.Evaluate(in)
}

// Evaluate 实现 ports.RiskEvaluator。
func (s *StaticRiskEvaluator) Evaluate(ctx context.Context, rc *ports.RiskContext) (*risk.Verdict, error) {
	_ = ctx
	return EvaluateRules(s.Rules, rc), nil
}
