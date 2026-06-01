package market

import (
	"context"
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/shopspring/decimal"
)

const perpQuoteCcy = "USDT"

// PerpRiskOMS 供 perp 风控读取 OMS 挂单数。
type PerpRiskOMS interface {
	NonFinalOrderCount() int
}

// PerpRiskEquity 供 perp 风控读取日亏与回撤。
type PerpRiskEquity interface {
	DailyLoss() decimal.Decimal
	DrawdownRatio() decimal.Decimal
}

// PerpRiskNotionalGate 由 Engine 实现，判断是否需要计算名义字段。
type PerpRiskNotionalGate interface {
	NeedsNotional(increasingExposure bool) bool
}

// PerpRiskConfig perp MarketRisk 依赖。
type PerpRiskConfig struct {
	Proj          *projection.AccountProjection
	Decision      *decision.DecisionEngine
	Execution     PerpRiskOMS
	Equity        PerpRiskEquity
	NeedsNotional PerpRiskNotionalGate
}

type perpRisk struct {
	proj          *projection.AccountProjection
	decision      *decision.DecisionEngine
	execution     PerpRiskOMS
	equity        PerpRiskEquity
	needsNotional PerpRiskNotionalGate
}

// NewPerpRisk 构造 perp 的 MarketRisk。
func NewPerpRisk(cfg PerpRiskConfig) MarketRisk {
	return &perpRisk{
		proj:          cfg.Proj,
		decision:      cfg.Decision,
		execution:     cfg.Execution,
		equity:        cfg.Equity,
		needsNotional: cfg.NeedsNotional,
	}
}

// BuildRiskContext 组装单笔订单的风控上下文（perp 行为与 v1 buildRiskContext 等价）。
func (p *perpRisk) BuildRiskContext(ctx context.Context, strategyName string, signal *models.Signal, order *models.Order) (*ports.RiskContext, error) {
	if order == nil {
		return nil, fmt.Errorf("nil order")
	}

	rc := &ports.RiskContext{
		StrategyName:  strategyName,
		Signal:        signal,
		Order:         order,
		OpensExposure: risk.OpensExposure(signal),
		QuoteCcy:      perpQuoteCcy,
	}

	if p.execution != nil {
		rc.OpenOrders = p.execution.NonFinalOrderCount()
	}

	if p.proj != nil {
		rc.ProjectionReady = p.proj.IsReady()
		if rc.ProjectionReady {
			if n, err := p.proj.OpenPositionCount(); err == nil {
				rc.Positions = n
			}
			if eq, err := p.proj.AccountEquity(); err == nil {
				rc.AccountEquity = eq
			}
		}
	}

	if p.equity != nil {
		rc.DailyLoss = p.equity.DailyLoss()
		rc.DrawdownRatio = p.equity.DrawdownRatio()
	}

	if !rc.OpensExposure {
		rc.NotionalAvailable = true
		return rc, nil
	}

	if !rc.ProjectionReady {
		return rc, nil
	}

	position, err := p.proj.PositionForContract(order.Symbol)
	if err != nil {
		return rc, nil
	}
	rc.IncreasingExposure = risk.IsIncreasingExposure(signal, position)

	needsNotional := false
	if p.needsNotional != nil {
		needsNotional = p.needsNotional.NeedsNotional(rc.IncreasingExposure)
	}
	if !needsNotional {
		rc.NotionalAvailable = true
		return rc, nil
	}

	if p.decision == nil {
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

	orderNotional, err := p.decision.NotionalForContracts(ctx, order.Symbol, orderQty, posMark)
	if err != nil {
		return rc, nil
	}

	rc.NotionalAvailable = true
	rc.OrderNotional = orderNotional
	rc.NotionalPerUnit = orderNotional.Div(orderQty)

	if position != nil {
		posQty, _ := decimal.NewFromString(strings.TrimSpace(position.Size))
		rc.PositionNotional, err = p.decision.NotionalForContracts(ctx, order.Symbol, posQty, position.MarkPrice)
		if err != nil {
			rc.NotionalAvailable = false
			return rc, nil
		}
	}

	rc.TotalExposure, err = p.totalExposure(ctx)
	if err != nil {
		rc.NotionalAvailable = false
		return rc, nil
	}

	if rc.IncreasingExposure {
		rc.PostPositionNotional = rc.PositionNotional.Add(orderNotional)
		rc.PostTotalExposure = rc.TotalExposure.Add(orderNotional)
	} else {
		rc.PostPositionNotional = rc.PositionNotional
		rc.PostTotalExposure = rc.TotalExposure
	}

	if rc.AccountEquity.Sign() > 0 {
		exposure := rc.PostTotalExposure
		if !rc.IncreasingExposure {
			exposure = rc.TotalExposure
		}
		rc.PostLeverage = risk.LeverageRatio(exposure, rc.AccountEquity)
	}

	return rc, nil
}

func (p *perpRisk) totalExposure(ctx context.Context) (decimal.Decimal, error) {
	if p.proj == nil {
		return decimal.Zero, fmt.Errorf("projection not configured")
	}
	if p.decision == nil {
		return decimal.Zero, fmt.Errorf("decision engine not configured")
	}
	positions, err := p.proj.AllPositions()
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
		n, err := p.decision.NotionalForContracts(ctx, pv.Contract, qty, pv.MarkPrice)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(n)
	}
	return total, nil
}

var _ MarketRisk = (*perpRisk)(nil)
