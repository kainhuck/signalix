package strategy

import (
	"fmt"

	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/shopspring/decimal"
)

// StrategyRiskConfigRaw 策略 config.yaml 可选 risk 段（指针字段区分未设置与显式零值）。
type StrategyRiskConfigRaw struct {
	MaxOrderSize       *float64 `yaml:"max_order_size"`
	MaxOpenOrders      *int     `yaml:"max_open_orders"`
	MaxPositions       *int     `yaml:"max_positions"`
	MaxPositionSize    *float64 `yaml:"max_position_size"`
	MaxDailyLoss       *float64 `yaml:"max_daily_loss"`
	MaxDrawdown        *float64 `yaml:"max_drawdown"`
	MaxLeverage        *int     `yaml:"max_leverage"`
	EnablePositionLock *bool    `yaml:"enable_position_lock"`
}

func validateRiskConfig(r *StrategyRiskConfigRaw) error {
	if r == nil {
		return nil
	}
	if r.MaxOrderSize != nil && *r.MaxOrderSize < 0 {
		return fmt.Errorf("risk.max_order_size must be non-negative")
	}
	if r.MaxOpenOrders != nil && *r.MaxOpenOrders < 0 {
		return fmt.Errorf("risk.max_open_orders must be non-negative")
	}
	if r.MaxPositions != nil && *r.MaxPositions < 0 {
		return fmt.Errorf("risk.max_positions must be non-negative")
	}
	if r.MaxPositionSize != nil && *r.MaxPositionSize < 0 {
		return fmt.Errorf("risk.max_position_size must be non-negative")
	}
	if r.MaxDailyLoss != nil && *r.MaxDailyLoss < 0 {
		return fmt.Errorf("risk.max_daily_loss must be non-negative")
	}
	if r.MaxDrawdown != nil {
		if *r.MaxDrawdown <= 0 || *r.MaxDrawdown > 1 {
			return fmt.Errorf("risk.max_drawdown must be in (0, 1]")
		}
	}
	if r.MaxLeverage != nil && *r.MaxLeverage < 0 {
		return fmt.Errorf("risk.max_leverage must be non-negative")
	}
	return nil
}

// RiskOverridesFromRaw 将 YAML risk 段转为 domain Overrides；无有效字段时返回 nil。
func RiskOverridesFromRaw(r *StrategyRiskConfigRaw) *risk.Overrides {
	if r == nil {
		return nil
	}
	o := &risk.Overrides{}
	if r.MaxOrderSize != nil {
		d := decimal.NewFromFloat(*r.MaxOrderSize)
		o.MaxOrderSize = &d
	}
	if r.MaxOpenOrders != nil {
		o.MaxOpenOrders = r.MaxOpenOrders
	}
	if r.MaxPositions != nil {
		o.MaxPositions = r.MaxPositions
	}
	if r.MaxPositionSize != nil {
		d := decimal.NewFromFloat(*r.MaxPositionSize)
		o.MaxPositionSize = &d
	}
	if r.MaxDailyLoss != nil {
		d := decimal.NewFromFloat(*r.MaxDailyLoss)
		o.MaxDailyLoss = &d
	}
	if r.MaxDrawdown != nil {
		d := decimal.NewFromFloat(*r.MaxDrawdown)
		o.MaxDrawdown = &d
	}
	if r.MaxLeverage != nil {
		o.MaxLeverage = r.MaxLeverage
	}
	if r.EnablePositionLock != nil {
		o.EnablePositionLock = r.EnablePositionLock
	}
	if !risk.HasEffectiveOverrides(*o) {
		return nil
	}
	return o
}
