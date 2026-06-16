package spot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/kainhuck/signalix/pkg/logger"
)

const (
	wsChannelSpotTickers      = "spot.tickers"
	wsChannelSpotCandlesticks = "spot.candlesticks"
)

type RouterOption func(*MarketRouter)

func WithRouterBuffer(n int) RouterOption {
	return func(mr *MarketRouter) {
		if mr != nil && n > 0 {
			mr.marketCh = make(chan market.MarketUpdate, n)
		}
	}
}

func WithRouterKlineHistoryMax(n int) RouterOption {
	return func(mr *MarketRouter) {
		if mr != nil && n > 0 {
			mr.klineHistMax = n
		}
	}
}

type candleKey struct {
	Pair     spotex.Pair
	Interval string
}

type MarketRouter struct {
	exchange ports.SpotExchange
	pairMeta map[spotex.Pair]*spotex.PairMeta

	tickerRef      map[spotex.Pair]int
	tickerPushSubs map[spotex.Pair]map[string]bool
	candleSubs     map[candleKey]map[string]bool
	subMu          sync.RWMutex

	tickerCache map[spotex.Pair]*spotex.TickerSnapshot
	tickerMu    sync.RWMutex

	klineCache map[candleKey]*spotex.CandlestickSnapshot
	klineMu    sync.RWMutex

	klineHistory map[candleKey][]*models.Kline
	klineHistMu  sync.RWMutex
	klineHistMax int

	marketCh chan market.MarketUpdate

	ctx    context.Context
	cancel context.CancelFunc
}

func NewMarketRouter(exchange ports.SpotExchange, pairMeta map[spotex.Pair]*spotex.PairMeta, opts ...RouterOption) *MarketRouter {
	ctx, cancel := context.WithCancel(context.Background())
	mr := &MarketRouter{
		exchange:       exchange,
		pairMeta:       clonePairMetaMap(pairMeta),
		tickerRef:      make(map[spotex.Pair]int),
		tickerPushSubs: make(map[spotex.Pair]map[string]bool),
		candleSubs:     make(map[candleKey]map[string]bool),
		tickerCache:    make(map[spotex.Pair]*spotex.TickerSnapshot),
		klineCache:     make(map[candleKey]*spotex.CandlestickSnapshot),
		klineHistory:   make(map[candleKey][]*models.Kline),
		klineHistMax:   DefaultKlineHistoryMax,
		marketCh:       make(chan market.MarketUpdate, DefaultMarketBuffer),
		ctx:            ctx,
		cancel:         cancel,
	}
	for _, opt := range opts {
		opt(mr)
	}
	return mr
}

func (mr *MarketRouter) Start() {
	logger.InfoContext(mr.ctx, "Spot market router started")
	for {
		select {
		case ev := <-mr.exchange.PublicEvents():
			mr.OnPublicEvent(ev)
		case <-mr.ctx.Done():
			return
		}
	}
}

func (mr *MarketRouter) Stop() error {
	mr.cancel()
	return nil
}

func (mr *MarketRouter) Subscribe(ctx context.Context, req market.SubscribeRequest) error {
	_ = ctx
	pairs, err := canonicalPairs(req.Symbols)
	if err != nil {
		return err
	}
	return mr.SubscribePairs(req.Strategy, pairs, req.Interval, req.PushTicker)
}

func (mr *MarketRouter) SubscribePairs(strategyName string, pairs []spotex.Pair, interval string, pushTicker bool) error {
	strategyName = strings.TrimSpace(strategyName)
	interval = strings.TrimSpace(interval)
	if strategyName == "" {
		return fmt.Errorf("strategy is required")
	}
	if interval == "" {
		return fmt.Errorf("interval is required")
	}

	mr.subMu.Lock()
	defer mr.subMu.Unlock()

	newTickerPairs := make([]spotex.Pair, 0, len(pairs))
	newCandleKeys := make([]candleKey, 0, len(pairs))
	for _, pair := range pairs {
		pair = pair.Canonical()
		if pair == "" {
			continue
		}
		if mr.tickerRef[pair] == 0 {
			newTickerPairs = append(newTickerPairs, pair)
		}
		mr.tickerRef[pair]++
		if pushTicker {
			if _, ok := mr.tickerPushSubs[pair]; !ok {
				mr.tickerPushSubs[pair] = make(map[string]bool)
			}
			mr.tickerPushSubs[pair][strategyName] = true
		}

		key := candleKey{Pair: pair, Interval: interval}
		if _, ok := mr.candleSubs[key]; !ok {
			mr.candleSubs[key] = make(map[string]bool)
			newCandleKeys = append(newCandleKeys, key)
		}
		mr.candleSubs[key][strategyName] = true
	}

	subs := make([]*spotex.Subscription, 0, 1+len(newCandleKeys))
	if len(newTickerPairs) > 0 {
		subs = append(subs, &spotex.Subscription{Channel: wsChannelSpotTickers, Pairs: newTickerPairs})
	}
	for _, key := range newCandleKeys {
		subs = append(subs, &spotex.Subscription{
			Channel: wsChannelSpotCandlesticks,
			Pairs:   []spotex.Pair{key.Pair},
			Payload: []string{key.Interval},
		})
	}
	if len(subs) == 0 {
		return nil
	}
	return mr.exchange.Subscribe(mr.ctx, subs)
}

func (mr *MarketRouter) Unsubscribe(strategyName string) error {
	mr.subMu.Lock()
	defer mr.subMu.Unlock()

	unsubTickers := mr.decTickerRefForStrategy(strategyName)
	unsubCandles := make([]*spotex.Subscription, 0)
	for key, strategies := range mr.candleSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(mr.candleSubs, key)
			unsubCandles = append(unsubCandles, &spotex.Subscription{
				Channel: wsChannelSpotCandlesticks,
				Pairs:   []spotex.Pair{key.Pair},
				Payload: []string{key.Interval},
			})
		}
	}
	mr.removeTickerPushForStrategy(strategyName)
	return mr.exchangeUnsubscribe(unsubTickers, unsubCandles)
}

func (mr *MarketRouter) decTickerRefForStrategy(strategyName string) []spotex.Pair {
	seen := make(map[spotex.Pair]bool)
	for key, strategies := range mr.candleSubs {
		if strategies[strategyName] {
			seen[key.Pair] = true
		}
	}
	unsub := make([]spotex.Pair, 0, len(seen))
	for pair := range seen {
		n, ok := mr.tickerRef[pair]
		if !ok || n <= 0 {
			continue
		}
		if n == 1 {
			delete(mr.tickerRef, pair)
			unsub = append(unsub, pair)
		} else {
			mr.tickerRef[pair] = n - 1
		}
	}
	return unsub
}

func (mr *MarketRouter) removeTickerPushForStrategy(strategyName string) {
	for pair, strategies := range mr.tickerPushSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(mr.tickerPushSubs, pair)
		}
	}
}

func (mr *MarketRouter) exchangeUnsubscribe(tickers []spotex.Pair, candles []*spotex.Subscription) error {
	subs := make([]*spotex.Subscription, 0, 1+len(candles))
	if len(tickers) > 0 {
		subs = append(subs, &spotex.Subscription{Channel: wsChannelSpotTickers, Pairs: tickers})
	}
	subs = append(subs, candles...)
	if len(subs) == 0 {
		return nil
	}
	return mr.exchange.Unsubscribe(mr.ctx, subs)
}

func (mr *MarketRouter) Updates() <-chan market.MarketUpdate {
	return mr.marketCh
}

func (mr *MarketRouter) OnPublicEvent(ev *spotex.PublicEvent) {
	if ev == nil {
		return
	}
	switch ev.Kind {
	case spotex.PublicTicker:
		mr.onTicker(ev)
	case spotex.PublicCandlestick:
		mr.onCandlestick(ev)
	}
}

func (mr *MarketRouter) onTicker(ev *spotex.PublicEvent) {
	ticker, ok := ev.Ticker()
	if !ok || ticker.Pair == "" {
		return
	}
	pair := ticker.Pair.Canonical()
	cp := *ticker
	cp.Pair = pair

	mr.tickerMu.Lock()
	mr.tickerCache[pair] = &cp
	mr.tickerMu.Unlock()

	mr.subMu.RLock()
	strategies, exists := mr.tickerPushSubs[pair]
	if !exists || len(strategies) == 0 {
		mr.subMu.RUnlock()
		return
	}
	strategyList := copyStrategyNames(strategies)
	mr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		mr.emit(market.MarketUpdate{
			Market:       models.MarketSpot,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateTicker,
			Ticker:       tickerFromSpot(&cp),
		})
	}
}

func (mr *MarketRouter) onCandlestick(ev *spotex.PublicEvent) {
	snap, ok := ev.Candlestick()
	if !ok || snap.Pair == "" || snap.Interval == "" {
		return
	}
	pair := snap.Pair.Canonical()
	cp := *snap
	cp.Pair = pair
	key := candleKey{Pair: pair, Interval: cp.Interval}

	mr.klineMu.Lock()
	mr.klineCache[key] = &cp
	mr.klineMu.Unlock()
	mr.appendClosedKline(&cp)

	if !cp.WindowClosed {
		return
	}
	mr.subMu.RLock()
	strategies, exists := mr.candleSubs[key]
	if !exists || len(strategies) == 0 {
		mr.subMu.RUnlock()
		return
	}
	strategyList := copyStrategyNames(strategies)
	mr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		mr.emit(market.MarketUpdate{
			Market:       models.MarketSpot,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateKline,
			Kline:        klineFromSpot(&cp),
		})
	}
}

func copyStrategyNames(strategies map[string]bool) []string {
	out := make([]string, 0, len(strategies))
	for name := range strategies {
		out = append(out, name)
	}
	return out
}

func (mr *MarketRouter) emit(upd market.MarketUpdate) {
	select {
	case mr.marketCh <- upd:
	case <-mr.ctx.Done():
	default:
		logger.WarnContext(mr.ctx, "Spot market channel full, dropping update",
			logger.String("strategy", upd.StrategyName),
			logger.Any("kind", upd.Kind))
	}
}

func (mr *MarketRouter) GetCachedTicker(pair spotex.Pair) (*spotex.TickerSnapshot, bool) {
	mr.tickerMu.RLock()
	defer mr.tickerMu.RUnlock()
	ticker, ok := mr.tickerCache[pair.Canonical()]
	if !ok {
		return nil, false
	}
	cp := *ticker
	return &cp, true
}

func (mr *MarketRouter) GetAllCachedTickers() map[spotex.Pair]*spotex.TickerSnapshot {
	mr.tickerMu.RLock()
	defer mr.tickerMu.RUnlock()
	out := make(map[spotex.Pair]*spotex.TickerSnapshot, len(mr.tickerCache))
	for pair, ticker := range mr.tickerCache {
		if ticker == nil {
			continue
		}
		cp := *ticker
		out[pair] = &cp
	}
	return out
}

func canonicalPairs(symbols []string) ([]spotex.Pair, error) {
	out := make([]spotex.Pair, 0, len(symbols))
	for _, sym := range symbols {
		pair := spotex.CanonicalPair(sym)
		if pair == "" {
			return nil, fmt.Errorf("symbol is required")
		}
		out = append(out, pair)
	}
	return out, nil
}

func tickerFromSpot(snap *spotex.TickerSnapshot) *models.Ticker {
	if snap == nil {
		return nil
	}
	return &models.Ticker{
		Contract:        perp.Contract(snap.Pair.Canonical().String()),
		Last:            snap.Last,
		MarkPrice:       snap.Last,
		ChangePct24h:    snap.ChangePct24h,
		Volume24h:       snap.Volume24hQuote,
		Volume24hBase:   snap.Volume24hBase,
		Volume24hQuote:  snap.Volume24hQuote,
		Low24h:          snap.Low24h,
		High24h:         snap.High24h,
		TimestampMillis: snap.TimestampMillis,
	}
}

func klineFromSpot(snap *spotex.CandlestickSnapshot) *models.Kline {
	if snap == nil {
		return nil
	}
	return &models.Kline{
		Contract:     perp.Contract(snap.Pair.Canonical().String()),
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

func clonePairMetaMap(in map[spotex.Pair]*spotex.PairMeta) map[spotex.Pair]*spotex.PairMeta {
	out := make(map[spotex.Pair]*spotex.PairMeta, len(in))
	for pair, meta := range in {
		if meta == nil {
			continue
		}
		cp := *meta
		cp.Pair = pair.Canonical()
		out[cp.Pair] = &cp
	}
	return out
}

func staleTicker(ticker *spotex.TickerSnapshot, maxAge time.Duration) bool {
	if ticker == nil || ticker.TimestampMillis == 0 {
		return true
	}
	return time.Since(time.UnixMilli(ticker.TimestampMillis)) > maxAge
}
