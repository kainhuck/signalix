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
		Rules:                    s.Rules,
		OrderSize:                rc.Order.Size,
		SignalOpensExposure:      risk.OpensExposure(rc.Signal),
		OpenOrderCount:           rc.OpenOrders,
		OpenPositionCount:        rc.Positions,
		ProjectionReady:          rc.ProjectionReady,
		NotionalAvailable:        rc.NotionalAvailable,
		OpensExposure:            rc.OpensExposure,
		IncreasingExposure:       rc.IncreasingExposure,
		OrderNotionalUSDT:        rc.OrderNotionalUSDT,
		PositionNotionalUSDT:     rc.PositionNotionalUSDT,
		PostPositionNotionalUSDT: rc.PostPositionNotionalUSDT,
		TotalExposureUSDT:        rc.TotalExposureUSDT,
		PostTotalExposureUSDT:    rc.PostTotalExposureUSDT,
		AccountEquityUSDT:        rc.AccountEquityUSDT,
		DailyLossUSDT:            rc.DailyLossUSDT,
		DrawdownRatio:            rc.DrawdownRatio,
		PostLeverage:             rc.PostLeverage,
		NotionalUSDTPerContract:  rc.NotionalUSDTPerContract,
	}
	return risk.Evaluate(in), nil
}

func signalOpensExposure(sig *models.Signal) bool {
	return risk.OpensExposure(sig)
}
