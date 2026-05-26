package projection

import (
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/shopspring/decimal"
)

// EquityHook 账户总权益变更回调（USDT）。
type EquityHook func(equity decimal.Decimal, at time.Time)

// RefreshHook REST 全量 refresh 成功后调用（持锁外执行；payload 为副本）。
type RefreshHook func(balance *perp.BalanceView, positions []*perp.PositionSnapshot, revision uint64, at time.Time)

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

// WithRefreshHook 注册 REST refresh 成功钩子。
func WithRefreshHook(h RefreshHook) Option {
	return func(p *AccountProjection) {
		if p != nil {
			p.refreshHook = h
		}
	}
}
