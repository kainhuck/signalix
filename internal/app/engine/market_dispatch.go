package engine

import (
	"fmt"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/models"
)

func (e *Engine) marketFor(m models.Market) market.Market {
	if e == nil || !m.Valid() {
		return nil
	}
	return e.markets[m]
}

func (e *Engine) strategyMarket(name string) models.Market {
	if st, ok := e.GetStrategy(name); ok && st != nil && st.Market.Valid() {
		return st.Market
	}
	return models.MarketPerp
}

func (e *Engine) pumpMarketUpdates(kind models.Market, m market.Market) {
	if e == nil || m == nil {
		return
	}
	for {
		select {
		case upd, ok := <-m.Updates():
			if !ok {
				return
			}
			if upd.Market == "" {
				upd.Market = kind
			}
			switch upd.Kind {
			case market.MarketUpdateTicker:
				e.dispatchTickerUpdate(upd)
			case market.MarketUpdateKline:
				e.dispatchKlineUpdate(upd)
			}
		case <-e.ctx.Done():
			return
		}
	}
}

func (e *Engine) subscribeStrategy(st *strategy.Strategy, interval string) error {
	if st == nil {
		return fmt.Errorf("nil strategy")
	}
	mk := st.Market
	if !mk.Valid() {
		mk = models.MarketPerp
	}
	m := e.marketFor(mk)
	if m == nil {
		return fmt.Errorf("market %q not registered for strategy %q", mk, st.Name)
	}
	return m.Subscribe(e.ctx, market.SubscribeRequest{
		Strategy:   st.Name,
		Symbols:    contractsToStrings(st.Symbols),
		Interval:   interval,
		PushTicker: st.SubscribeTicker,
	})
}

func (e *Engine) unsubscribeStrategy(name string) {
	mk := e.strategyMarket(name)
	if m := e.marketFor(mk); m != nil {
		_ = m.Unsubscribe(name)
	}
}
