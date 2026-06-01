package decision

import (
	"fmt"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/shopspring/decimal"
)

// ComputeUSDTNotionalFromSignal 根据 signal 的 sizing 配置从可用余额计算 USDT 名义金额。
// sizing_mode 为 nil 时：availableBalance / defaultDivisor（divisor<=0 时按 10）。
// Custom 模式不在此处理，由 calculateOrderSize 直接解析张数。
func ComputeUSDTNotionalFromSignal(
	signal *models.Signal,
	availableBalance decimal.Decimal,
	defaultDivisor int,
) (decimal.Decimal, error) {
	if signal == nil {
		return decimal.Zero, fmt.Errorf("invalid signal")
	}

	divisor := defaultDivisor
	if divisor <= 0 {
		divisor = 10
	}

	if signal.SizingMode == nil {
		return availableBalance.Div(decimal.NewFromInt(int64(divisor))), nil
	}

	switch *signal.SizingMode {
	case models.SizingModePercent:
		if signal.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for percent sizing mode")
		}
		percent, _ := decimal.NewFromString(*signal.Value)
		if percent.IsNegative() || percent.GreaterThan(decimal.NewFromFloat(1.0)) {
			return decimal.Zero, fmt.Errorf("percent must be between 0 and 1, got: %s", *signal.Value)
		}
		return availableBalance.Mul(percent), nil

	case models.SizingModeFixed:
		if signal.Value == nil {
			return decimal.Zero, fmt.Errorf("value is required for fixed sizing mode")
		}
		fixedSize, _ := decimal.NewFromString(*signal.Value)
		if fixedSize.GreaterThan(availableBalance) {
			return decimal.Zero, fmt.Errorf("fixed size %s exceeds available balance %s", *signal.Value, availableBalance.String())
		}
		return fixedSize, nil

	case models.SizingModeCustom:
		return decimal.Zero, fmt.Errorf("custom sizing mode must be handled by caller")

	default:
		return decimal.Zero, fmt.Errorf("unknown sizing mode: %s", *signal.SizingMode)
	}
}
