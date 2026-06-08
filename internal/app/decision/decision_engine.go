package decision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// DecisionEngine 决策引擎
// 负责将策略信号转换为交易订单
type DecisionEngine struct {
	acct               *projection.AccountProjection
	metaLookup         instrument.ContractMetaLookup
	tickerLookup       TickerLookup
	defaultSizeDivisor int
	mu                 sync.RWMutex
	stats              DecisionStats
}

// DecisionStats 决策统计
type DecisionStats struct {
	SignalsProcessed int64
	SignalsRejected  int64
	OrdersGenerated  int64
}

// NewDecisionEngine 创建决策引擎；projection 不可为 nil（由 Engine 注入）。
func NewDecisionEngine(acct *projection.AccountProjection, opts ...Option) *DecisionEngine {
	de := &DecisionEngine{
		acct:               acct,
		defaultSizeDivisor: 10,
		stats:              DecisionStats{},
	}
	for _, o := range opts {
		o(de)
	}
	return de
}

// Decide 满足 market.MarketDecider 语义（与 ProcessSignal 等价）。
func (de *DecisionEngine) Decide(ctx context.Context, strategyName string, signal *models.Signal) (*models.Order, error) {
	return de.processSignal(ctx, strategyName, signal)
}

// ProcessSignal 处理策略信号，转换为订单（保留公开 API）。
func (de *DecisionEngine) ProcessSignal(ctx context.Context, strategyName string, signal *models.Signal) (*models.Order, error) {
	return de.processSignal(ctx, strategyName, signal)
}

func (de *DecisionEngine) processSignal(ctx context.Context, strategyName string, signal *models.Signal) (*models.Order, error) {
	de.mu.Lock()
	de.stats.SignalsProcessed++
	de.mu.Unlock()

	if err := signal.Validate(); err != nil {
		de.rejectSignal()
		return nil, fmt.Errorf("invalid signal: %w", err)
	}

	if de.acct == nil {
		de.rejectSignal()
		return nil, fmt.Errorf("account projection is not configured")
	}
	balance, position, _, err := de.acct.Snapshot(perp.Contract(signal.Symbol))
	if err != nil {
		de.rejectSignal()
		return nil, fmt.Errorf("account snapshot: %w", err)
	}

	// 计算订单大小（张数）
	size, err := de.calculateOrderSize(ctx, signal, balance, position)
	if err != nil {
		de.rejectSignal()
		return nil, fmt.Errorf("failed to calculate order size: %w", err)
	}

	// 如果 size 为 0，说明不需要下单（例如已经持有目标仓位或没有持仓可平）
	if size.IsZero() {
		return nil, nil
	}

	// 确定订单方向
	side, err := de.determineOrderSide(signal, position)
	if err != nil {
		de.rejectSignal()
		return nil, fmt.Errorf("failed to determine order side: %w", err)
	}

	// 如果 side 为空，说明不需要下单
	if side == "" {
		return nil, nil
	}

	// 创建订单
	order := &models.Order{
		ID:           generateOrderID(),
			Symbol:       signal.Symbol,
			Side:         side,
			OrderType:    models.OrderTypeMarket,
			Size:         de.formatSizeForOrder(ctx, perp.Contract(signal.Symbol), size),
		FilledSize:   "0",
		Status:       models.OrderStatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		StrategyName: strategyName,
	}

	// 如果信号指定了价格，使用限价单
	if signal.Price != nil {
		price, _ := decimal.NewFromString(*signal.Price)
		if price.IsPositive() {
			order.OrderType = models.OrderTypeLimit
			order.Price = signal.Price
		}
	}

	de.mu.Lock()
	de.stats.OrdersGenerated++
	de.mu.Unlock()

	return order, nil
}

func (de *DecisionEngine) formatSizeForOrder(_ context.Context, contract perp.Contract, size decimal.Decimal) string {
	if de.metaLookup == nil {
		return size.String()
	}
	meta, err := de.metaLookup.ContractMeta(contract)
	if err != nil {
		return size.String()
	}
	return formatOrderSize(size, meta)
}

// calculateOrderSize 计算订单大小（Gate 永续张数）。
func (de *DecisionEngine) calculateOrderSize(ctx context.Context, signal *models.Signal, balance *perp.BalanceView, position *perp.PositionSnapshot) (decimal.Decimal, error) {
	if signal == nil || balance == nil {
		return decimal.Zero, fmt.Errorf("invalid signal or balance")
	}

	// 如果信号是平仓
	if signal.Direction == models.DirectionFlat {

		if position == nil {
			return decimal.Zero, nil
		}

		size, _ := decimal.NewFromString(position.Size)

		return size, nil
	}

	positionSize := decimal.Zero
	if position != nil {
		positionSize, _ = decimal.NewFromString(position.Size)
	}

	// 如果已经持有相同方向的仓位，不重复开仓
	if position != nil && positionSize.IsPositive() {
		if (signal.Direction == models.DirectionLong && position.Side == perp.PositionLong) ||
			(signal.Direction == models.DirectionShort && position.Side == perp.PositionShort) {
			return decimal.Zero, nil
		}
	}

	availableBalance, _ := decimal.NewFromString(balance.Available)

	if signal.SizingMode != nil && *signal.SizingMode == models.SizingModeCustom {
		if signal.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for custom sizing mode")
		}
		value, _ := decimal.NewFromString(*signal.Value)
		if !value.IsPositive() {
			return decimal.Zero, fmt.Errorf("custom size must be positive")
		}
		return value, nil
	}

	notional, err := ComputeUSDTNotionalFromSignal(signal, availableBalance, de.defaultSizeDivisor)
	if err != nil {
		return decimal.Zero, err
	}
	return de.notionalUSDTToContracts(ctx, perp.Contract(signal.Symbol), notional)
}

// determineOrderSide 确定订单方向
func (de *DecisionEngine) determineOrderSide(signal *models.Signal, position *perp.PositionSnapshot) (models.OrderSide, error) {
	switch signal.Direction {
	case models.DirectionLong:
		return models.OrderSideBuy, nil

	case models.DirectionShort:
		return models.OrderSideSell, nil

	case models.DirectionFlat:
		// 平仓：根据当前持仓方向决定
		if position == nil {
			return "", nil // 没有持仓时返回空，不是错误
		}

		positionSize, _ := decimal.NewFromString(position.Size)
		if positionSize.IsZero() {
			return "", nil // 没有持仓时买入开多
		}

		if position.Side == perp.PositionLong {
			return models.OrderSideSell, nil // 卖出平多
		}
		return models.OrderSideBuy, nil // 买入平空

	default:
		return "", fmt.Errorf("invalid direction: %s", signal.Direction)
	}
}

// GetStats 获取统计信息
func (de *DecisionEngine) GetStats() DecisionStats {
	de.mu.RLock()
	defer de.mu.RUnlock()
	return de.stats
}

// ResetStats 重置统计信息
func (de *DecisionEngine) ResetStats() {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.stats = DecisionStats{}
}

func (de *DecisionEngine) rejectSignal() {
	de.mu.Lock()
	de.stats.SignalsRejected++
	de.mu.Unlock()
}

// generateOrderID 生成订单 ID
func generateOrderID() string {
	return uuid.New().String()
}
