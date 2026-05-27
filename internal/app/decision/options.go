package decision

import (
	"github.com/kainhuck/signalix/internal/app/instrument"
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

// WithContractMetaLookup 注入进程内合约元数据 lookup（Engine Registry）。
func WithContractMetaLookup(lookup instrument.ContractMetaLookup) Option {
	return func(de *DecisionEngine) {
		if de != nil {
			de.metaLookup = lookup
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
