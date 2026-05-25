package risk

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Input 纯函数入参（由 engines 从 RiskContext 映射而来）。
type Input struct {
	Rules Rules

	OrderSize           string
	SignalOpensExposure bool
	OpenOrderCount      int
	OpenPositionCount   int // -1 表示未知，跳过 max_positions
}

// Evaluate 执行规则链；始终返回非 nil Verdict。
func Evaluate(in Input) *Verdict {
	if !in.Rules.Enable {
		return Allow()
	}

	qty, err := decimal.NewFromString(in.OrderSize)
	if err != nil || qty.Sign() <= 0 {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_INVALID_ORDER_SIZE",
			Message: fmt.Sprintf("invalid order size %q", in.OrderSize),
		}
	}

	if in.Rules.MaxOpenOrders > 0 && in.OpenOrderCount >= in.Rules.MaxOpenOrders {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_MAX_OPEN_ORDERS",
			Message: fmt.Sprintf("open orders %d >= limit %d", in.OpenOrderCount, in.Rules.MaxOpenOrders),
		}
	}

	if in.Rules.MaxPositions > 0 && in.SignalOpensExposure && in.OpenPositionCount >= 0 &&
		in.OpenPositionCount >= in.Rules.MaxPositions {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_MAX_POSITIONS",
			Message: fmt.Sprintf("position slots %d >= limit %d", in.OpenPositionCount, in.Rules.MaxPositions),
		}
	}

	if !in.Rules.MaxOrderSize.IsZero() && qty.GreaterThan(in.Rules.MaxOrderSize) {
		return &Verdict{
			Kind:         KindReduce,
			Code:         "RISK_MAX_ORDER_SIZE",
			Message:      fmt.Sprintf("order size %s exceeds max %s", qty.String(), in.Rules.MaxOrderSize.String()),
			AdjustedSize: in.Rules.MaxOrderSize.String(),
		}
	}

	return Allow()
}
