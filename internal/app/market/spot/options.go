package spot

import "github.com/kainhuck/signalix/internal/app/market"

type RouterOption func(*SpotRouter)

func WithBufferSize(n int) RouterOption {
	return func(sr *SpotRouter) {
		if sr != nil && n > 0 {
			sr.marketCh = make(chan market.MarketUpdate, n)
		}
	}
}
