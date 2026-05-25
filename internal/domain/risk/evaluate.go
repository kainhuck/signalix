package risk

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Input 纯函数入参（由 Engine 从 RiskContext 映射而来）。
type Input struct {
	Rules Rules

	OrderSize           string
	SignalOpensExposure bool
	OpenOrderCount      int
	OpenPositionCount   int // -1 表示未知，跳过 max_positions

	ProjectionReady    bool
	NotionalAvailable  bool
	OpensExposure      bool
	IncreasingExposure bool

	OrderNotionalUSDT        decimal.Decimal
	PositionNotionalUSDT     decimal.Decimal
	PostPositionNotionalUSDT decimal.Decimal
	TotalExposureUSDT        decimal.Decimal
	PostTotalExposureUSDT    decimal.Decimal
	AccountEquityUSDT        decimal.Decimal
	DailyLossUSDT            decimal.Decimal
	DrawdownRatio            decimal.Decimal
	PostLeverage             decimal.Decimal
	NotionalUSDTPerContract  decimal.Decimal
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

	if in.OpensExposure && !in.ProjectionReady {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_PROJECTION_NOT_READY",
			Message: "account projection not ready for opening exposure",
		}
	}

	if in.Rules.EnablePositionLock && in.OpensExposure {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_POSITION_LOCK",
			Message: "position lock enabled, opening exposure rejected",
		}
	}

	if in.OpensExposure && in.AccountEquityUSDT.Sign() <= 0 {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_INVALID_EQUITY",
			Message: "account equity must be positive for opening exposure",
		}
	}

	if in.OpensExposure && in.Rules.MaxDailyLoss.Sign() > 0 && in.DailyLossUSDT.GreaterThanOrEqual(in.Rules.MaxDailyLoss) {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_MAX_DAILY_LOSS",
			Message: fmt.Sprintf("daily loss %s >= limit %s", in.DailyLossUSDT.String(), in.Rules.MaxDailyLoss.String()),
		}
	}

	if in.OpensExposure && in.Rules.MaxDrawdown.Sign() > 0 && in.DrawdownRatio.GreaterThanOrEqual(in.Rules.MaxDrawdown) {
		return &Verdict{
			Kind:    KindReject,
			Code:    "RISK_MAX_DRAWDOWN",
			Message: fmt.Sprintf("drawdown %s >= limit %s", in.DrawdownRatio.String(), in.Rules.MaxDrawdown.String()),
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

	if in.OpensExposure && in.Rules.MaxLeverage > 0 {
		if !in.NotionalAvailable {
			return &Verdict{
				Kind:    KindReject,
				Code:    "RISK_NOTIONAL_UNAVAILABLE",
				Message: "notional metrics unavailable for leverage check",
			}
		}
		maxLev := decimal.NewFromInt(int64(in.Rules.MaxLeverage))
		if in.PostLeverage.GreaterThan(maxLev) {
			return &Verdict{
				Kind:    KindReject,
				Code:    "RISK_MAX_LEVERAGE",
				Message: fmt.Sprintf("post-trade leverage %s > limit %d", in.PostLeverage.StringFixed(2), in.Rules.MaxLeverage),
			}
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

	if in.IncreasingExposure && in.Rules.MaxPositionSize.Sign() > 0 {
		if !in.NotionalAvailable {
			return &Verdict{
				Kind:    KindReject,
				Code:    "RISK_NOTIONAL_UNAVAILABLE",
				Message: "notional metrics unavailable for position size check",
			}
		}
		if in.PostPositionNotionalUSDT.GreaterThan(in.Rules.MaxPositionSize) {
			allowedDelta := in.Rules.MaxPositionSize.Sub(in.PositionNotionalUSDT)
			if allowedDelta.Sign() <= 0 {
				return &Verdict{
					Kind:    KindReject,
					Code:    "RISK_MAX_POSITION_SIZE",
					Message: fmt.Sprintf("position notional %s already at or above limit %s", in.PositionNotionalUSDT.String(), in.Rules.MaxPositionSize.String()),
				}
			}
			if in.NotionalUSDTPerContract.Sign() <= 0 {
				return &Verdict{
					Kind:    KindReject,
					Code:    "RISK_NOTIONAL_UNAVAILABLE",
					Message: "cannot convert position limit to contract size",
				}
			}
			allowedQty := allowedDelta.Div(in.NotionalUSDTPerContract).Floor()
			if allowedQty.Sign() <= 0 {
				return &Verdict{
					Kind:    KindReject,
					Code:    "RISK_MAX_POSITION_SIZE",
					Message: "allowed contract size after position limit is zero",
				}
			}
			if qty.GreaterThan(allowedQty) {
				return &Verdict{
					Kind:         KindReduce,
					Code:         "RISK_MAX_POSITION_SIZE",
					Message:      fmt.Sprintf("post position notional %s exceeds max %s", in.PostPositionNotionalUSDT.String(), in.Rules.MaxPositionSize.String()),
					AdjustedSize: allowedQty.String(),
				}
			}
		}
	}

	return Allow()
}
