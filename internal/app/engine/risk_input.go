package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/shopspring/decimal"
)

func (e *Engine) buildRiskContext(ctx context.Context, strategyName string, signal *models.Signal, order *models.Order) (*ports.RiskContext, error) {
	if order == nil {
		return nil, fmt.Errorf("nil order")
	}

	rc := &ports.RiskContext{
		StrategyName:  strategyName,
		Signal:        signal,
		Order:         order,
		OpensExposure: risk.OpensExposure(signal),
	}

	if e.executionEngine != nil {
		rc.OpenOrders = e.executionEngine.NonFinalOrderCount()
	}

	if e.accountProjection != nil {
		rc.ProjectionReady = e.accountProjection.IsReady()
		if rc.ProjectionReady {
			if n, err := e.accountProjection.OpenPositionCount(); err == nil {
				rc.Positions = n
			}
			if eq, err := e.accountProjection.AccountEquity(); err == nil {
				rc.AccountEquityUSDT = eq
			}
		}
	}

	if e.equityTracker != nil {
		rc.DailyLossUSDT = e.equityTracker.DailyLoss()
		rc.DrawdownRatio = e.equityTracker.DrawdownRatio()
	}

	if !rc.OpensExposure {
		rc.NotionalAvailable = true
		return rc, nil
	}

	if !rc.ProjectionReady {
		return rc, nil
	}

	position, err := e.accountProjection.PositionForContract(order.Symbol)
	if err != nil {
		return rc, nil
	}
	rc.IncreasingExposure = risk.IsIncreasingExposure(signal, position)

	needsNotional := e.riskNeedsLeverage() || (rc.IncreasingExposure && e.riskNeedsPositionSize())
	if !needsNotional {
		rc.NotionalAvailable = true
		return rc, nil
	}

	posMark := ""
	if position != nil {
		posMark = position.MarkPrice
	}

	orderQty, err := decimal.NewFromString(strings.TrimSpace(order.Size))
	if err != nil || !orderQty.IsPositive() {
		return rc, nil
	}

	orderNotional, err := e.decisionEngine.NotionalForContracts(ctx, order.Symbol, orderQty, posMark)
	if err != nil {
		return rc, nil
	}

	rc.NotionalAvailable = true
	rc.OrderNotionalUSDT = orderNotional
	rc.NotionalUSDTPerContract = orderNotional.Div(orderQty)

	if position != nil {
		posQty, _ := decimal.NewFromString(strings.TrimSpace(position.Size))
		rc.PositionNotionalUSDT, err = e.decisionEngine.NotionalForContracts(ctx, order.Symbol, posQty, position.MarkPrice)
		if err != nil {
			rc.NotionalAvailable = false
			return rc, nil
		}
	}

	rc.TotalExposureUSDT, err = e.totalExposureUSDT(ctx)
	if err != nil {
		rc.NotionalAvailable = false
		return rc, nil
	}

	if rc.IncreasingExposure {
		rc.PostPositionNotionalUSDT = rc.PositionNotionalUSDT.Add(orderNotional)
		rc.PostTotalExposureUSDT = rc.TotalExposureUSDT.Add(orderNotional)
	} else {
		rc.PostPositionNotionalUSDT = rc.PositionNotionalUSDT
		rc.PostTotalExposureUSDT = rc.TotalExposureUSDT
	}

	if rc.AccountEquityUSDT.Sign() > 0 {
		exposure := rc.PostTotalExposureUSDT
		if !rc.IncreasingExposure {
			exposure = rc.TotalExposureUSDT
		}
		rc.PostLeverage = risk.LeverageRatio(exposure, rc.AccountEquityUSDT)
	}

	return rc, nil
}

func (e *Engine) totalExposureUSDT(ctx context.Context) (decimal.Decimal, error) {
	if e.accountProjection == nil {
		return decimal.Zero, fmt.Errorf("projection not configured")
	}
	positions, err := e.accountProjection.AllPositions()
	if err != nil {
		return decimal.Zero, err
	}
	total := decimal.Zero
	for _, pv := range positions {
		if pv == nil {
			continue
		}
		qty, err := decimal.NewFromString(strings.TrimSpace(pv.Size))
		if err != nil || !qty.IsPositive() {
			continue
		}
		n, err := e.decisionEngine.NotionalForContracts(ctx, pv.Contract, qty, pv.MarkPrice)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(n)
	}
	return total, nil
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
