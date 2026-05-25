package decision

import (
	"github.com/kainhuck/signalix/internal/ports"
)

// Option 决策引擎可选配置。
type Option func(*DecisionEngine)

// WithDefaultSizeDivisor 无 sizing mode 时：可用余额 / divisor。
func WithDefaultSizeDivisor(divisor int) Option {
	return func(de *DecisionEngine) {
		if de != nil && divisor > 0 {
			de.defaultSizeDivisor = divisor
		}
	}
}

// WithExchange 用于加载合约元数据（quanto_multiplier 等）。
func WithExchange(ex ports.Exchange) Option {
	return func(de *DecisionEngine) {
		if de != nil {
			de.exchange = ex
		}
	}
}

// WithTickerLookup 用于 USDT 名义 → 张数换算的标记价（引擎 ticker 缓存）。
func WithTickerLookup(lookup TickerLookup) Option {
	return func(de *DecisionEngine) {
		if de != nil {
			de.tickerLookup = lookup
		}
	}
}
