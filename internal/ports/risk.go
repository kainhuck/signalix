package ports

import (
	"context"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
)

// RiskContext 单笔订单过风控时的只读上下文（由 Engine 组装）。
type RiskContext struct {
	StrategyName string
	Signal       *models.Signal
	Order        *models.Order

	// OpenOrders：当前 OMS 内未终态订单数（不含本笔尚未入表）。
	OpenOrders int
	// Positions：账户投影中有仓位的合约条数；-1 表示未知（投影未就绪等）。
	Positions int
}

// RiskEvaluator 风控评估端口（实现可为静态规则、gRPC 远程等）。
type RiskEvaluator interface {
	Evaluate(ctx context.Context, rc *RiskContext) (*risk.Verdict, error)
}
