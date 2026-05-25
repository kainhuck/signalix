package risk

import "github.com/shopspring/decimal"

// Rules 风控规则快照（可由 YAML 或代码填充）。
type Rules struct {
	Enable        bool
	MaxOrderSize  decimal.Decimal // 零值：不限制单笔数量
	MaxOpenOrders int             // 0：不限制未完结单数
	MaxPositions  int             // 0：不限制持仓合约条数
}

// DefaultRules 与 config.example.yaml 中 risk 段数量级对齐；单笔为与 Order.Size 相同单位的绝对上限。
func DefaultRules() Rules {
	return Rules{
		Enable:        true,
		MaxOrderSize:  decimal.NewFromInt(100000),
		MaxOpenOrders: 50,
		MaxPositions:  20,
	}
}
