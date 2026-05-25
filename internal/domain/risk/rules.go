package risk

import "github.com/shopspring/decimal"

// Rules 风控规则快照（可由 YAML 或代码填充）。
type Rules struct {
	Enable             bool
	MaxOrderSize       decimal.Decimal // 张数；零值：不限制
	MaxOpenOrders      int             // 0：不限制
	MaxPositions       int             // 0：不限制
	MaxPositionSize    decimal.Decimal // USDT 名义；零：不限
	MaxDailyLoss       decimal.Decimal // USDT；零：不限
	MaxDrawdown        decimal.Decimal // 0~1；零：不限
	MaxLeverage        int             // 0：不限
	EnablePositionLock bool
}

// DefaultRules 与 config.example.toml 中 risk 段数量级对齐。
func DefaultRules() Rules {
	return Rules{
		Enable:        true,
		MaxOrderSize:  decimal.NewFromInt(100000),
		MaxOpenOrders: 50,
		MaxPositions:  20,
	}
}
