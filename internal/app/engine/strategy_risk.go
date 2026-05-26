package engine

import (
	"context"
	"strings"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
	"github.com/shopspring/decimal"
)

func (e *Engine) globalRules() risk.Rules {
	if ev, ok := e.riskEvaluator.(*StaticRiskEvaluator); ok {
		return ev.Rules
	}
	return risk.DefaultRules()
}

func (e *Engine) evaluateStrategyRisk(ctx context.Context, strategyName string, signal *models.Signal, order *models.Order, base *ports.RiskContext) (*risk.Verdict, *ports.RiskContext, error) {
	st, ok := e.GetStrategy(strategyName)
	if !ok || st == nil || st.RiskOverrides == nil || !risk.HasEffectiveOverrides(*st.RiskOverrides) {
		return risk.Allow(), base, nil
	}

	merged := risk.MergeRules(e.globalRules(), *st.RiskOverrides)
	scoped, err := e.buildStrategyRiskContext(base, strategyName, st.Symbols)
	if err != nil {
		return nil, base, err
	}

	v := EvaluateRules(merged, scoped)
	v = risk.PrefixStrategyVerdict(v)
	if v.Kind == risk.KindReduce {
		order.Size = v.AdjustedSize
		rebuilt, err := e.buildRiskContext(ctx, strategyName, signal, order)
		if err != nil {
			return v, base, err
		}
		return v, rebuilt, nil
	}
	return v, base, nil
}

func (e *Engine) buildStrategyRiskContext(base *ports.RiskContext, strategyName string, symbols []perp.Contract) (*ports.RiskContext, error) {
	if base == nil {
		return nil, nil
	}
	scoped := *base
	if e.executionEngine != nil {
		scoped.OpenOrders = e.executionEngine.NonFinalOrderCountForStrategy(strategyName)
	}
	pos, err := e.openPositionCountForStrategy(symbols)
	if err != nil {
		return nil, err
	}
	scoped.Positions = pos
	return &scoped, nil
}

func (e *Engine) openPositionCountForStrategy(symbols []perp.Contract) (int, error) {
	if e.accountProjection == nil || !e.accountProjection.IsReady() {
		return -1, nil
	}
	if len(symbols) == 0 {
		return 0, nil
	}
	allowed := make(map[perp.Contract]struct{}, len(symbols))
	for _, s := range symbols {
		allowed[s] = struct{}{}
	}
	positions, err := e.accountProjection.AllPositions()
	if err != nil {
		return -1, err
	}
	n := 0
	for _, pv := range positions {
		if pv == nil {
			continue
		}
		if _, ok := allowed[pv.Contract]; !ok {
			continue
		}
		qty, err := decimal.NewFromString(strings.TrimSpace(pv.Size))
		if err != nil || !qty.IsPositive() {
			continue
		}
		n++
	}
	return n, nil
}

func (e *Engine) logRiskReject(strategyName string, tier string, order *models.Order, v *risk.Verdict) {
	if order == nil {
		logger.WarnContext(e.ctx, "order rejected by risk",
			logger.String("code", v.Code),
			logger.String("strategy", strategyName),
			logger.String("risk_tier", tier),
			logger.String("message", v.Message))
		return
	}
	logger.WarnContext(e.ctx, "order rejected by risk",
		logger.String("code", v.Code),
		logger.String("strategy", strategyName),
		logger.String("risk_tier", tier),
		logger.String("order_id", order.ID),
		logger.String("symbol", string(order.Symbol)),
		logger.String("message", v.Message))
}
