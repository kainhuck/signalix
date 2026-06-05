package spot

import (
	"context"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	spgate "github.com/kainhuck/signalix/pkg/exchange/spot/gateio"
	"github.com/kainhuck/signalix/pkg/logger"
)

const (
	wsChannelTickers      = spgate.WSChannelTickers
	wsChannelCandlesticks = spgate.WSChannelCandlesticks

	defaultKlineHistoryMax = 2000
)

type SpotRouter struct {
	client *spgate.Client

	tickerRef      map[spot.Pair]int
	tickerPushSubs map[spot.Pair]map[string]bool
	candleSubs     map[pairCandleKey]map[string]bool
	subMu          sync.RWMutex

	tickerCache map[spot.Pair]*spot.TickerSnapshot
	tickerMu    sync.RWMutex

	klineCache map[pairCandleKey]*spot.CandlestickSnapshot
	klineMu    sync.RWMutex

	klineHistory map[pairCandleKey][]*models.Kline
	klineHistMu  sync.RWMutex
	klineHistMax int

	marketCh chan market.MarketUpdate

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSpotRouter(client *spgate.Client, opts ...RouterOption) *SpotRouter {
	ctx, cancel := context.WithCancel(context.Background())

	sr := &SpotRouter{
		client:         client,
		tickerRef:      make(map[spot.Pair]int),
		tickerPushSubs: make(map[spot.Pair]map[string]bool),
		candleSubs:     make(map[pairCandleKey]map[string]bool),
		tickerCache:    make(map[spot.Pair]*spot.TickerSnapshot),
		klineCache:     make(map[pairCandleKey]*spot.CandlestickSnapshot),
		klineHistory:   make(map[pairCandleKey][]*models.Kline),
		klineHistMax:   defaultKlineHistoryMax,
		marketCh:       make(chan market.MarketUpdate, 1000),
		ctx:            ctx,
		cancel:         cancel,
	}
	for _, o := range opts {
		o(sr)
	}
	return sr
}

func (sr *SpotRouter) Start() {
	logger.InfoContext(sr.ctx, "Spot Market Router started")

	for {
		select {
		case ev := <-sr.client.PublicEvents():
			sr.OnPublicEvent(ev)
		case <-sr.ctx.Done():
			return
		}
	}
}

func (sr *SpotRouter) Stop() error {
	logger.InfoContext(sr.ctx, "Stopping Spot Market Router...")
	sr.cancel()
	logger.InfoContext(sr.ctx, "Spot Market Router stopped")
	return nil
}

func (sr *SpotRouter) OnPublicEvent(ev *spot.PublicEvent) {
	if ev == nil {
		return
	}
	switch ev.Kind {
	case spot.PublicTicker:
		sr.onTicker(ev)
	case spot.PublicCandlestick:
		sr.onCandlestick(ev)
	}
}

func (sr *SpotRouter) SubscribePairs(strategyName string, pairs []spot.Pair, interval string, subscribeTicker bool) error {
	sr.subMu.Lock()
	defer sr.subMu.Unlock()

	newTickerPairs := make([]spot.Pair, 0, len(pairs))
	newCandleKeys := make([]pairCandleKey, 0, len(pairs))

	for _, p := range pairs {
		if sr.tickerRef[p] == 0 {
			newTickerPairs = append(newTickerPairs, p)
		}
		sr.tickerRef[p]++
		if subscribeTicker {
			if _, exists := sr.tickerPushSubs[p]; !exists {
				sr.tickerPushSubs[p] = make(map[string]bool)
			}
			sr.tickerPushSubs[p][strategyName] = true
		}

		key := pairCandleKey{Pair: p, Interval: interval}
		if _, exists := sr.candleSubs[key]; !exists {
			sr.candleSubs[key] = make(map[string]bool)
			newCandleKeys = append(newCandleKeys, key)
		}
		sr.candleSubs[key][strategyName] = true
	}

	subs := make([]*spot.Subscription, 0, 2)
	if len(newTickerPairs) > 0 {
		subs = append(subs, &spot.Subscription{
			Channel: wsChannelTickers,
			Pairs:   newTickerPairs,
		})
	}
	for _, key := range newCandleKeys {
		subs = append(subs, &spot.Subscription{
			Channel: wsChannelCandlesticks,
			Pairs:   []spot.Pair{key.Pair},
			Payload: []string{key.Interval},
		})
	}
	if len(subs) == 0 {
		logger.InfoContext(sr.ctx, "Spot Subscribe success (no new exchange subs)",
			logger.String("strategy", strategyName),
			logger.Any("pairs", pairs),
			logger.String("interval", interval),
			logger.Any("subscribe_ticker", subscribeTicker))
		return nil
	}

	if err := sr.client.Subscribe(sr.ctx, subs); err != nil {
		return err
	}

	logger.InfoContext(sr.ctx, "Spot Subscribe success",
		logger.String("strategy", strategyName),
		logger.Any("pairs", pairs),
		logger.String("interval", interval),
		logger.Any("subscribe_ticker", subscribeTicker))
	return nil
}

func (sr *SpotRouter) UnsubscribePairs(strategyName string, pairs []spot.Pair, interval string) error {
	sr.subMu.Lock()
	defer sr.subMu.Unlock()

	sr.removeTickerPushSubs(strategyName, pairs)
	unsubTickers := sr.decTickerRefForPairs(strategyName, pairs)
	unsubCandles := sr.removeCandleSubs(strategyName, pairs, interval)

	return sr.exchangeUnsubscribe(unsubTickers, unsubCandles)
}

func (sr *SpotRouter) UnsubscribeAll(strategyName string) error {
	sr.subMu.Lock()
	defer sr.subMu.Unlock()

	unsubTickers := sr.decTickerRefForPairs(strategyName, nil)

	var unsubCandles []*spot.Subscription
	for key, strategies := range sr.candleSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(sr.candleSubs, key)
			unsubCandles = append(unsubCandles, &spot.Subscription{
				Channel: wsChannelCandlesticks,
				Pairs:   []spot.Pair{key.Pair},
				Payload: []string{key.Interval},
			})
		}
	}
	sr.removeTickerPushForStrategy(strategyName)

	if err := sr.exchangeUnsubscribe(unsubTickers, unsubCandles); err != nil {
		return err
	}

	logger.InfoContext(sr.ctx, "Spot Unsubscribed all success", logger.String("strategy", strategyName))
	return nil
}

func (sr *SpotRouter) removeTickerPushSubs(strategyName string, pairs []spot.Pair) {
	for _, p := range pairs {
		if strategies, exists := sr.tickerPushSubs[p]; exists {
			delete(strategies, strategyName)
			if len(strategies) == 0 {
				delete(sr.tickerPushSubs, p)
			}
		}
	}
}

func (sr *SpotRouter) removeTickerPushForStrategy(strategyName string) {
	for p, strategies := range sr.tickerPushSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(sr.tickerPushSubs, p)
		}
	}
}

func (sr *SpotRouter) decTickerRefForPairs(strategyName string, pairs []spot.Pair) []spot.Pair {
	var targets []spot.Pair
	if len(pairs) > 0 {
		targets = pairs
	} else {
		seen := make(map[spot.Pair]bool)
		for key, strategies := range sr.candleSubs {
			if strategies[strategyName] && !seen[key.Pair] {
				seen[key.Pair] = true
				targets = append(targets, key.Pair)
			}
		}
	}
	var unsub []spot.Pair
	for _, p := range targets {
		n, ok := sr.tickerRef[p]
		if !ok || n <= 0 {
			continue
		}
		if n == 1 {
			delete(sr.tickerRef, p)
			unsub = append(unsub, p)
		} else {
			sr.tickerRef[p] = n - 1
		}
	}
	return unsub
}

func (sr *SpotRouter) removeCandleSubs(strategyName string, pairs []spot.Pair, interval string) []*spot.Subscription {
	var unsub []*spot.Subscription
	for _, p := range pairs {
		key := pairCandleKey{Pair: p, Interval: interval}
		strategies, exists := sr.candleSubs[key]
		if !exists {
			continue
		}
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(sr.candleSubs, key)
			unsub = append(unsub, &spot.Subscription{
				Channel: wsChannelCandlesticks,
				Pairs:   []spot.Pair{key.Pair},
				Payload: []string{key.Interval},
			})
		}
	}
	return unsub
}

func (sr *SpotRouter) exchangeUnsubscribe(tickerPairs []spot.Pair, candleSubs []*spot.Subscription) error {
	subs := make([]*spot.Subscription, 0, 1+len(candleSubs))
	if len(tickerPairs) > 0 {
		subs = append(subs, &spot.Subscription{
			Channel: wsChannelTickers,
			Pairs:   tickerPairs,
		})
	}
	subs = append(subs, candleSubs...)
	if len(subs) == 0 {
		return nil
	}
	return sr.client.Unsubscribe(sr.ctx, subs)
}

func (sr *SpotRouter) onTicker(ev *spot.PublicEvent) {
	snap, ok := ev.Ticker()
	if !ok || snap.Pair == "" {
		return
	}
	p := snap.Pair

	sr.tickerMu.Lock()
	sr.tickerCache[p] = snap
	sr.tickerMu.Unlock()

	sr.subMu.RLock()
	strategies, exists := sr.tickerPushSubs[p]
	if !exists || len(strategies) == 0 {
		sr.subMu.RUnlock()
		return
	}
	strategyList := sr.copyStrategyNames(strategies)
	sr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		sr.emit(market.MarketUpdate{
			Market:       models.MarketSpot,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateTicker,
			Ticker:       spotTickerToModel(snap),
		})
	}
}

func (sr *SpotRouter) onCandlestick(ev *spot.PublicEvent) {
	snap, ok := ev.Candlestick()
	if !ok || snap.Pair == "" || snap.Interval == "" {
		return
	}
	key := pairCandleKey{Pair: snap.Pair, Interval: snap.Interval}

	sr.klineMu.Lock()
	cp := *snap
	sr.klineCache[key] = &cp
	sr.klineMu.Unlock()

	sr.appendClosedKline(snap)

	if !snap.WindowClosed {
		return
	}

	sr.subMu.RLock()
	strategies, exists := sr.candleSubs[key]
	if !exists || len(strategies) == 0 {
		sr.subMu.RUnlock()
		return
	}
	strategyList := sr.copyStrategyNames(strategies)
	sr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		sr.emit(market.MarketUpdate{
			Market:       models.MarketSpot,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateKline,
			Kline:        spotKlineToModel(snap),
		})
	}
}

func (sr *SpotRouter) copyStrategyNames(strategies map[string]bool) []string {
	out := make([]string, 0, len(strategies))
	for name := range strategies {
		out = append(out, name)
	}
	return out
}

func (sr *SpotRouter) emit(upd market.MarketUpdate) {
	select {
	case sr.marketCh <- upd:
	case <-sr.ctx.Done():
	default:
		logger.WarnContext(sr.ctx, "Channel full, dropping spot market data",
			logger.String("strategy", upd.StrategyName),
			logger.Any("kind", upd.Kind))
	}
}

func (sr *SpotRouter) GetCachedTicker(pair spot.Pair) (*spot.TickerSnapshot, bool) {
	sr.tickerMu.RLock()
	defer sr.tickerMu.RUnlock()

	snap, exists := sr.tickerCache[pair]
	if !exists || snap == nil {
		return nil, false
	}
	cp := *snap
	return &cp, true
}

func (sr *SpotRouter) GetAllCachedTickers() map[spot.Pair]*spot.TickerSnapshot {
	sr.tickerMu.RLock()
	defer sr.tickerMu.RUnlock()

	result := make(map[spot.Pair]*spot.TickerSnapshot, len(sr.tickerCache))
	for p, snap := range sr.tickerCache {
		if snap == nil {
			continue
		}
		cp := *snap
		result[p] = &cp
	}
	return result
}

func (sr *SpotRouter) GetSubscribedPairs(strategyName string) []spot.Pair {
	sr.subMu.RLock()
	defer sr.subMu.RUnlock()

	var pairs []spot.Pair
	seen := make(map[spot.Pair]bool)
	for key, strategies := range sr.candleSubs {
		if strategies[strategyName] && !seen[key.Pair] {
			seen[key.Pair] = true
			pairs = append(pairs, key.Pair)
		}
	}
	return pairs
}

func (sr *SpotRouter) GetCacheSize() int {
	sr.tickerMu.RLock()
	defer sr.tickerMu.RUnlock()
	return len(sr.tickerCache)
}

func (sr *SpotRouter) CleanStaleCache(maxAge time.Duration) int {
	sr.tickerMu.Lock()
	defer sr.tickerMu.Unlock()

	now := time.Now()
	removed := 0
	for p, snap := range sr.tickerCache {
		if snap == nil {
			delete(sr.tickerCache, p)
			removed++
			continue
		}
		if now.Sub(time.UnixMilli(snap.TimestampMillis)) > maxAge {
			delete(sr.tickerCache, p)
			removed++
		}
	}
	if removed > 0 {
		logger.InfoContext(sr.ctx, "Cleaned stale spot cache entries", logger.Int("removed", removed))
	}
	return removed
}

func spotTickerToModel(snap *spot.TickerSnapshot) *models.Ticker {
	if snap == nil {
		return nil
	}
	return &models.Ticker{
		Contract:        perp.Contract(string(snap.Pair)),
		Last:            snap.Last,
		ChangePct24h:    snap.ChangePct24h,
		Volume24hBase:   snap.Volume24hBase,
		Volume24hQuote:  snap.Volume24hQuote,
		Low24h:          snap.Low24h,
		High24h:         snap.High24h,
		TimestampMillis: snap.TimestampMillis,
	}
}

func spotKlineToModel(snap *spot.CandlestickSnapshot) *models.Kline {
	if snap == nil {
		return nil
	}
	return &models.Kline{
		Contract:     perp.Contract(string(snap.Pair)),
		Interval:     snap.Interval,
		Open:         snap.Open,
		High:         snap.High,
		Low:          snap.Low,
		Close:        snap.Close,
		Volume:       snap.Volume,
		VolumeBase:   snap.VolumeBase,
		TimestampSec: snap.TimestampSec,
		WindowClosed: snap.WindowClosed,
	}
}

func pairsFromStrings(ss []string) []spot.Pair {
	out := make([]spot.Pair, len(ss))
	for i, s := range ss {
		out[i] = spot.CanonicalPair(s)
	}
	return out
}
