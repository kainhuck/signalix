package spot

import (
	"context"

	"github.com/kainhuck/signalix/internal/app/market"
)

var _ market.MarketFeed = (*SpotRouter)(nil)

func (sr *SpotRouter) Subscribe(ctx context.Context, req market.SubscribeRequest) error {
	_ = ctx
	return sr.SubscribePairs(req.Strategy, pairsFromStrings(req.Symbols), req.Interval, req.PushTicker)
}

func (sr *SpotRouter) Unsubscribe(strategy string) error {
	return sr.UnsubscribeAll(strategy)
}

func (sr *SpotRouter) Updates() <-chan market.MarketUpdate {
	return sr.marketCh
}
