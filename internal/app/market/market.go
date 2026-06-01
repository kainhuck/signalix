package market

import (
	"context"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// Market 引擎对"一个已接入市场"的统一抽象。引擎层只认它，不关心 perp/spot。
// 一个具体市场实现同时满足下列全部小接口。
type Market interface {
	// Kind 返回市场类型（models.MarketPerp / models.MarketSpot）。
	Kind() models.Market

	// Start 启动市场自有的后台工作（私有流消费、projection 刷新等）。
	Start(ctx context.Context) error
	// Stop 停止市场后台工作。
	Stop() error

	MarketFeed
	MarketDecider
	MarketExecutor
	MarketRisk
	MarketAccount
}

// MarketDecider 将中性信号翻译为中性订单。
type MarketDecider interface {
	// Decide 将策略信号转换为订单；返回 nil 表示无需下单（如已持仓、size 为 0）。
	Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error)
}

// MarketExecutor 供 OMS 使用的执行翻译；无状态，不持有订单状态机。
type MarketExecutor interface {
	// Place 把中性订单翻译为所侧请求并下单一次，返回交易所订单号。
	Place(ctx context.Context, o *models.Order) (exchangeID string, err error)
	// Cancel 撤销订单。
	Cancel(ctx context.Context, o *models.Order) error
	// Sync 主动查询所侧最新状态，返回归一化结果供 OMS 更新本地状态机。
	Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error)
	// OrderEvents 归一化用户订单事件流；实现内部在 emit 前已更新自身 projection。
	OrderEvents() <-chan *models.OrderEvent
}

// MarketRisk 组装市场相关的只读风控上下文；判定逻辑仍在 domain/risk（市场无关）。
type MarketRisk interface {
	BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error)
}

// MarketAccount 账户与行情只读查询，供策略 RPC / gRPC。
type MarketAccount interface {
	Balance(ctx context.Context, currency string) (*models.BalanceView, error)
	Position(ctx context.Context, symbol string) (*models.PositionView, error)
	Ticker(symbol string) (*models.Ticker, error)
	Klines(symbol, interval string, limit int) ([]*models.Kline, error)
}
