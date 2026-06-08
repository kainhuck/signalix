package spot

import (
	"context"
	"fmt"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/shopspring/decimal"
)

type SpotRiskOMS interface {
	NonFinalOrderCount() int
}

type SpotRiskEquity interface {
	DailyLoss() decimal.Decimal
	DrawdownRatio() decimal.Decimal
}

type spotRisk struct {
	execution SpotRiskOMS
	equity    SpotRiskEquity
}

func NewSpotRisk(execution SpotRiskOMS, equity SpotRiskEquity) market.MarketRisk {
	return &spotRisk{
		execution: execution,
		equity:    equity,
	}
}

func (r *spotRisk) BuildRiskContext(ctx context.Context, strategyName string, signal *models.Signal, order *models.Order) (*ports.RiskContext, error) {
	if order == nil {
		return nil, fmt.Errorf("nil order")
	}

	pair := spot.CanonicalPair(string(order.Symbol))
	quoteCcy := pair.QuoteCurrency()
	if quoteCcy == "" {
		quoteCcy = "USDT"
	}

	rc := &ports.RiskContext{
		StrategyName:     strategyName,
		Signal:           signal,
		Order:            order,
		OpensExposure:    signal != nil && signal.Direction != models.DirectionFlat,
		QuoteCcy:         quoteCcy,
		ProjectionReady:  true,
		NotionalAvailable: true,
		AccountEquity:    decimal.NewFromInt(1),
	}

	if r.execution != nil {
		rc.OpenOrders = r.execution.NonFinalOrderCount()
	}

	if r.equity != nil {
		rc.DailyLoss = r.equity.DailyLoss()
		rc.DrawdownRatio = r.equity.DrawdownRatio()
	}

	return rc, nil
}

var _ market.MarketRisk = (*spotRisk)(nil)
