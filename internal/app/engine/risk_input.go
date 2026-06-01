package engine

import (
	"context"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

func (e *Engine) buildRiskContext(ctx context.Context, strategyName string, signal *models.Signal, order *models.Order) (*ports.RiskContext, error) {
	return e.perpRisk.BuildRiskContext(ctx, strategyName, signal, order)
}

// NeedsNotional 实现 market.PerpRiskNotionalGate。
func (e *Engine) NeedsNotional(increasingExposure bool) bool {
	return e.riskNeedsLeverage() || (increasingExposure && e.riskNeedsPositionSize())
}

func (e *Engine) riskNeedsLeverage() bool {
	if rulesNeedLeverage(e.globalRules()) {
		return true
	}
	e.strategyMu.RLock()
	defer e.strategyMu.RUnlock()
	global := e.globalRules()
	for _, st := range e.strategies {
		if st == nil || st.RiskOverrides == nil || !risk.HasEffectiveOverrides(*st.RiskOverrides) {
			continue
		}
		if rulesNeedLeverage(risk.MergeRules(global, *st.RiskOverrides)) {
			return true
		}
	}
	return false
}

func (e *Engine) riskNeedsPositionSize() bool {
	if rulesNeedPositionSize(e.globalRules()) {
		return true
	}
	e.strategyMu.RLock()
	defer e.strategyMu.RUnlock()
	global := e.globalRules()
	for _, st := range e.strategies {
		if st == nil || st.RiskOverrides == nil || !risk.HasEffectiveOverrides(*st.RiskOverrides) {
			continue
		}
		if rulesNeedPositionSize(risk.MergeRules(global, *st.RiskOverrides)) {
			return true
		}
	}
	return false
}

func rulesNeedLeverage(r risk.Rules) bool {
	return r.MaxLeverage > 0
}

func rulesNeedPositionSize(r risk.Rules) bool {
	return r.MaxPositionSize.Sign() > 0
}
