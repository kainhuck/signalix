package perp

import "github.com/kainhuck/signalix/internal/app/market"

// Option 行情路由器可选配置。
type Option func(*MarketRouter)

// WithBufferSize 设置 marketCh 缓冲容量。
func WithBufferSize(n int) Option {
	return func(mr *MarketRouter) {
		if mr != nil && n > 0 {
			mr.marketCh = make(chan market.MarketUpdate, n)
		}
	}
}
