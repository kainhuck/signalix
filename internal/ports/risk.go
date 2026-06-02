package ports

import (
	"context"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/shopspring/decimal"
)

// RiskContext 单笔订单过风控时的只读上下文（由 Engine / MarketRisk 组装）。
type RiskContext struct {
	StrategyName string
	Signal       *models.Signal
	Order        *models.Order

	OpenOrders int
	Positions  int

	ProjectionReady    bool
	NotionalAvailable  bool
	OpensExposure      bool
	IncreasingExposure bool

	QuoteCcy string // 计价币种；perp 为 USDT

	OrderNotional        decimal.Decimal
	PositionNotional     decimal.Decimal
	PostPositionNotional decimal.Decimal
	TotalExposure        decimal.Decimal
	PostTotalExposure    decimal.Decimal
	AccountEquity        decimal.Decimal
	DailyLoss            decimal.Decimal
	DrawdownRatio        decimal.Decimal
	PostLeverage         decimal.Decimal
	NotionalPerUnit      decimal.Decimal
}

// RiskEvaluator 风控评估端口（实现可为静态规则、gRPC 远程等）。
type RiskEvaluator interface {
	Evaluate(ctx context.Context, rc *RiskContext) (*risk.Verdict, error)
}
