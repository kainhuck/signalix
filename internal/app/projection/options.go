package projection

import (
	"time"

	"github.com/shopspring/decimal"
)

// EquityHook 账户总权益变更回调（USDT）。
type EquityHook func(equity decimal.Decimal, at time.Time)

// Option 账户投影可选配置。
type Option func(*AccountProjection)

// WithRefreshInterval 设置 REST 校准周期。
func WithRefreshInterval(d time.Duration) Option {
	return func(p *AccountProjection) {
		if p != nil && d > 0 {
			p.refreshInterval = d
		}
	}
}

// WithEquityHook 注册权益变更钩子（REST refresh / WS 余额更新后触发）。
func WithEquityHook(h EquityHook) Option {
	return func(p *AccountProjection) {
		if p != nil {
			p.equityHook = h
		}
	}
}
