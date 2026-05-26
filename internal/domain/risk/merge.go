package risk

import "github.com/shopspring/decimal"

// Overrides 策略 config.yaml risk 段（仅非 nil 指针字段参与覆盖）。
type Overrides struct {
	MaxOrderSize       *decimal.Decimal
	MaxOpenOrders      *int
	MaxPositions       *int
	MaxPositionSize    *decimal.Decimal
	MaxDailyLoss       *decimal.Decimal
	MaxDrawdown        *decimal.Decimal
	MaxLeverage        *int
	EnablePositionLock *bool
}

// HasEffectiveOverrides 是否存在至少一个 override 字段（含显式零值）。
func HasEffectiveOverrides(overrides Overrides) bool {
	return overrides.MaxOrderSize != nil ||
		overrides.MaxOpenOrders != nil ||
		overrides.MaxPositions != nil ||
		overrides.MaxPositionSize != nil ||
		overrides.MaxDailyLoss != nil ||
		overrides.MaxDrawdown != nil ||
		overrides.MaxLeverage != nil ||
		overrides.EnablePositionLock != nil
}

// MergeRules 返回策略级有效规则：overrides 已设字段覆盖 global，其余继承 global。
func MergeRules(global Rules, overrides Overrides) Rules {
	out := global
	if overrides.MaxOrderSize != nil {
		out.MaxOrderSize = *overrides.MaxOrderSize
	}
	if overrides.MaxOpenOrders != nil {
		out.MaxOpenOrders = *overrides.MaxOpenOrders
	}
	if overrides.MaxPositions != nil {
		out.MaxPositions = *overrides.MaxPositions
	}
	if overrides.MaxPositionSize != nil {
		out.MaxPositionSize = *overrides.MaxPositionSize
	}
	if overrides.MaxDailyLoss != nil {
		out.MaxDailyLoss = *overrides.MaxDailyLoss
	}
	if overrides.MaxDrawdown != nil {
		out.MaxDrawdown = *overrides.MaxDrawdown
	}
	if overrides.MaxLeverage != nil {
		out.MaxLeverage = *overrides.MaxLeverage
	}
	if overrides.EnablePositionLock != nil {
		out.EnablePositionLock = *overrides.EnablePositionLock
	}
	return out
}

// PrefixStrategyVerdict 为策略级 verdict 添加 STRATEGY_ 前缀（Allow 不变）。
func PrefixStrategyVerdict(v *Verdict) *Verdict {
	if v == nil || v.Kind == KindAllow {
		return v
	}
	code := v.Code
	if len(code) >= 5 && code[:5] == "RISK_" {
		code = "STRATEGY_" + code
	}
	return &Verdict{
		Kind:         v.Kind,
		Code:         code,
		Message:      v.Message,
		AdjustedSize: v.AdjustedSize,
	}
}
