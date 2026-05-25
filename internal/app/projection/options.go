package projection

import "time"

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
