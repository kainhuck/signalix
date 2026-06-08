package perp

import (
	"context"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

const (
	wsChannelTickers      = "futures.tickers"
	wsChannelCandlesticks = "futures.candlesticks"
)

type candleKey struct {
	Contract perp.Contract
	Interval string
}

// MarketRouter 行情数据路由器
type MarketRouter struct {
	exchange ports.PerpExchange

	tickerRef      map[perp.Contract]int             // 引擎侧 WS 订阅引用（与 subscribe_ticker 无关）
	tickerPushSubs map[perp.Contract]map[string]bool // subscribe_ticker=true 时向策略 fan-out
	candleSubs     map[candleKey]map[string]bool
	subMu          sync.RWMutex

	tickerCache map[perp.Contract]*perp.TickerSnapshot
	tickerMu    sync.RWMutex

	klineCache map[candleKey]*perp.CandlestickSnapshot
	klineMu    sync.RWMutex

	klineHistory map[candleKey][]*models.Kline
	klineHistMu  sync.RWMutex
	klineHistMax int

	marketCh chan market.MarketUpdate

	ctx    context.Context
	cancel context.CancelFunc
}

// NewMarketRouter 创建行情路由器
func NewMarketRouter(exchange ports.PerpExchange, opts ...Option) *MarketRouter {
	ctx, cancel := context.WithCancel(context.Background())

	mr := &MarketRouter{
		exchange:       exchange,
		tickerRef:      make(map[perp.Contract]int),
		tickerPushSubs: make(map[perp.Contract]map[string]bool),
		candleSubs:     make(map[candleKey]map[string]bool),
		tickerCache:    make(map[perp.Contract]*perp.TickerSnapshot),
		klineCache:     make(map[candleKey]*perp.CandlestickSnapshot),
		klineHistory:   make(map[candleKey][]*models.Kline),
		klineHistMax:   DefaultKlineHistoryMax,
		marketCh:       make(chan market.MarketUpdate, 1000),
		ctx:            ctx,
		cancel:         cancel,
	}
	for _, o := range opts {
		o(mr)
	}
	return mr
}

// Start 启动路由器
func (mr *MarketRouter) Start() {
	logger.InfoContext(mr.ctx, "Market Router started")

	for {
		select {
		case ev := <-mr.exchange.PublicEvents():
			mr.OnPublicEvent(ev)
		case <-mr.ctx.Done():
			return
		}
	}
}

// Stop 停止路由器
func (mr *MarketRouter) Stop() error {
	logger.InfoContext(mr.ctx, "Stopping Market Router...")
	mr.cancel()
	logger.InfoContext(mr.ctx, "Market Router stopped")
	return nil
}

// SubscribeContracts 订阅 K 线（perp 合约）；引擎始终订 ticker 写缓存，subscribeTicker 为 true 时额外向策略推送 tick。
func (mr *MarketRouter) SubscribeContracts(strategyName string, symbols []perp.Contract, interval string, subscribeTicker bool) error {
	mr.subMu.Lock()
	defer mr.subMu.Unlock()

	newTickerContracts := make([]perp.Contract, 0, len(symbols))
	newCandleKeys := make([]candleKey, 0, len(symbols))

	for _, symbol := range symbols {
		if mr.tickerRef[symbol] == 0 {
			newTickerContracts = append(newTickerContracts, symbol)
		}
		mr.tickerRef[symbol]++
		if subscribeTicker {
			if _, exists := mr.tickerPushSubs[symbol]; !exists {
				mr.tickerPushSubs[symbol] = make(map[string]bool)
			}
			mr.tickerPushSubs[symbol][strategyName] = true
		}

		key := candleKey{Contract: symbol, Interval: interval}
		if _, exists := mr.candleSubs[key]; !exists {
			mr.candleSubs[key] = make(map[string]bool)
			newCandleKeys = append(newCandleKeys, key)
		}
		mr.candleSubs[key][strategyName] = true
	}

	subs := make([]*perp.Subscription, 0, 2)
	if len(newTickerContracts) > 0 {
		subs = append(subs, &perp.Subscription{
			Channel:   wsChannelTickers,
			Contracts: newTickerContracts,
		})
	}
	for _, key := range newCandleKeys {
		subs = append(subs, &perp.Subscription{
			Channel:   wsChannelCandlesticks,
			Contracts: []perp.Contract{key.Contract},
			Payload:   []string{key.Interval},
		})
	}
	if len(subs) == 0 {
		logger.InfoContext(mr.ctx, "Subscribe success (no new exchange subs)",
			logger.String("strategy", strategyName),
			logger.Any("symbols", symbols),
			logger.String("interval", interval),
			logger.Any("subscribe_ticker", subscribeTicker))
		return nil
	}

	if err := mr.exchange.Subscribe(mr.ctx, subs); err != nil {
		return err
	}

	logger.InfoContext(mr.ctx, "Subscribe success",
		logger.String("strategy", strategyName),
		logger.Any("symbols", symbols),
		logger.String("interval", interval),
		logger.Any("subscribe_ticker", subscribeTicker))
	return nil
}

// UnsubscribeContracts 取消订阅（需与 SubscribeContracts 使用相同 interval）。
func (mr *MarketRouter) UnsubscribeContracts(strategyName string, symbols []perp.Contract, interval string) error {
	mr.subMu.Lock()
	defer mr.subMu.Unlock()

	mr.removeTickerPushSubs(strategyName, symbols)
	unsubTickers := mr.decTickerRefForSymbols(strategyName, symbols)
	unsubCandles := mr.removeCandleSubs(strategyName, symbols, interval)

	return mr.exchangeUnsubscribe(unsubTickers, unsubCandles)
}

// UnsubscribeAll 取消策略的所有订阅
func (mr *MarketRouter) UnsubscribeAll(strategyName string) error {
	mr.subMu.Lock()
	defer mr.subMu.Unlock()

	unsubTickers := mr.decTickerRefForSymbols(strategyName, nil)

	var unsubCandles []*perp.Subscription
	for key, strategies := range mr.candleSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(mr.candleSubs, key)
			unsubCandles = append(unsubCandles, &perp.Subscription{
				Channel:   wsChannelCandlesticks,
				Contracts: []perp.Contract{key.Contract},
				Payload:   []string{key.Interval},
			})
		}
	}
	mr.removeTickerPushForStrategy(strategyName)

	if err := mr.exchangeUnsubscribe(unsubTickers, unsubCandles); err != nil {
		return err
	}

	logger.InfoContext(mr.ctx, "Unsubscribed all success", logger.String("strategy", strategyName))
	return nil
}

func (mr *MarketRouter) removeTickerPushSubs(strategyName string, symbols []perp.Contract) {
	for _, symbol := range symbols {
		if strategies, exists := mr.tickerPushSubs[symbol]; exists {
			delete(strategies, strategyName)
			if len(strategies) == 0 {
				delete(mr.tickerPushSubs, symbol)
			}
		}
	}
}

func (mr *MarketRouter) removeTickerPushForStrategy(strategyName string) {
	for symbol, strategies := range mr.tickerPushSubs {
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(mr.tickerPushSubs, symbol)
		}
	}
}

// decTickerRefForSymbols 减少 ticker WS 引用；symbols 为 nil 时按 candleSubs 中该策略的合约减量。
func (mr *MarketRouter) decTickerRefForSymbols(strategyName string, symbols []perp.Contract) []perp.Contract {
	var contracts []perp.Contract
	if len(symbols) > 0 {
		contracts = symbols
	} else {
		seen := make(map[perp.Contract]bool)
		for key, strategies := range mr.candleSubs {
			if strategies[strategyName] && !seen[key.Contract] {
				seen[key.Contract] = true
				contracts = append(contracts, key.Contract)
			}
		}
	}
	var unsub []perp.Contract
	for _, symbol := range contracts {
		n, ok := mr.tickerRef[symbol]
		if !ok || n <= 0 {
			continue
		}
		if n == 1 {
			delete(mr.tickerRef, symbol)
			unsub = append(unsub, symbol)
		} else {
			mr.tickerRef[symbol] = n - 1
		}
	}
	return unsub
}

func (mr *MarketRouter) removeCandleSubs(strategyName string, symbols []perp.Contract, interval string) []*perp.Subscription {
	var unsub []*perp.Subscription
	for _, symbol := range symbols {
		key := candleKey{Contract: symbol, Interval: interval}
		strategies, exists := mr.candleSubs[key]
		if !exists {
			continue
		}
		delete(strategies, strategyName)
		if len(strategies) == 0 {
			delete(mr.candleSubs, key)
			unsub = append(unsub, &perp.Subscription{
				Channel:   wsChannelCandlesticks,
				Contracts: []perp.Contract{key.Contract},
				Payload:   []string{key.Interval},
			})
		}
	}
	return unsub
}

func (mr *MarketRouter) exchangeUnsubscribe(tickerContracts []perp.Contract, candleSubs []*perp.Subscription) error {
	subs := make([]*perp.Subscription, 0, 1+len(candleSubs))
	if len(tickerContracts) > 0 {
		subs = append(subs, &perp.Subscription{
			Channel:   wsChannelTickers,
			Contracts: tickerContracts,
		})
	}
	subs = append(subs, candleSubs...)
	if len(subs) == 0 {
		return nil
	}
	return mr.exchange.Unsubscribe(mr.ctx, subs)
}

// OnPublicEvent 处理公共行情事件。
func (mr *MarketRouter) OnPublicEvent(ev *perp.PublicEvent) {
	if ev == nil {
		return
	}
	switch ev.Kind {
	case perp.PublicTicker:
		mr.onTicker(ev)
	case perp.PublicCandlestick:
		mr.onCandlestick(ev)
	}
}

func (mr *MarketRouter) onTicker(ev *perp.PublicEvent) {
	ticker, ok := ev.Ticker()
	if !ok || ticker.Contract == "" {
		return
	}
	contract := ticker.Contract

	mr.tickerMu.Lock()
	mr.tickerCache[contract] = ticker
	mr.tickerMu.Unlock()

	mr.subMu.RLock()
	strategies, exists := mr.tickerPushSubs[contract]
	if !exists || len(strategies) == 0 {
		mr.subMu.RUnlock()
		return
	}
	strategyList := mr.copyStrategyNames(strategies)
	mr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		mr.emit(market.MarketUpdate{
			Market:       models.MarketPerp,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateTicker,
			Ticker:       models.TickerFromSnapshot(ticker),
		})
	}
}

func (mr *MarketRouter) onCandlestick(ev *perp.PublicEvent) {
	snap, ok := ev.Candlestick()
	if !ok || snap.Contract == "" || snap.Interval == "" {
		return
	}
	key := candleKey{Contract: snap.Contract, Interval: snap.Interval}

	mr.klineMu.Lock()
	cp := *snap
	mr.klineCache[key] = &cp
	mr.klineMu.Unlock()

	mr.appendClosedKline(snap)

	if !snap.WindowClosed {
		return
	}

	mr.subMu.RLock()
	strategies, exists := mr.candleSubs[key]
	if !exists || len(strategies) == 0 {
		mr.subMu.RUnlock()
		return
	}
	strategyList := mr.copyStrategyNames(strategies)
	mr.subMu.RUnlock()

	for _, strategyName := range strategyList {
		mr.emit(market.MarketUpdate{
			Market:       models.MarketPerp,
			StrategyName: strategyName,
			Kind:         market.MarketUpdateKline,
			Kline:        models.KlineFromSnapshot(snap),
		})
	}
}

func (mr *MarketRouter) copyStrategyNames(strategies map[string]bool) []string {
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
		logger.WarnContext(mr.ctx, "Channel full, dropping market data",
			logger.String("strategy", upd.StrategyName),
			logger.Any("kind", upd.Kind))
	}
}

// GetCachedTicker 获取缓存的 ticker。
func (mr *MarketRouter) GetCachedTicker(symbol perp.Contract) (*perp.TickerSnapshot, bool) {
	mr.tickerMu.RLock()
	defer mr.tickerMu.RUnlock()

	ticker, exists := mr.tickerCache[symbol]
	if !exists {
		return nil, false
	}
	cp := *ticker
	return &cp, true
}

// GetAllCachedTickers 获取所有缓存的 ticker。
func (mr *MarketRouter) GetAllCachedTickers() map[perp.Contract]*perp.TickerSnapshot {
	mr.tickerMu.RLock()
	defer mr.tickerMu.RUnlock()

	result := make(map[perp.Contract]*perp.TickerSnapshot, len(mr.tickerCache))
	for symbol, ticker := range mr.tickerCache {
		cp := *ticker
		result[symbol] = &cp
	}
	return result
}

// GetSubscribedSymbols 获取指定策略订阅的 symbols
func (mr *MarketRouter) GetSubscribedSymbols(strategyName string) []perp.Contract {
	mr.subMu.RLock()
	defer mr.subMu.RUnlock()

	var symbols []perp.Contract
	seen := make(map[perp.Contract]bool)
	for key, strategies := range mr.candleSubs {
		if strategies[strategyName] && !seen[key.Contract] {
			seen[key.Contract] = true
			symbols = append(symbols, key.Contract)
		}
	}
	return symbols
}

// GetAllSubscriptions 获取向策略推送 ticker 的订阅信息
func (mr *MarketRouter) GetAllSubscriptions() map[perp.Contract][]string {
	mr.subMu.RLock()
	defer mr.subMu.RUnlock()

	result := make(map[perp.Contract][]string)
	for symbol, strategies := range mr.tickerPushSubs {
		strategyList := make([]string, 0, len(strategies))
		for strategyName := range strategies {
			strategyList = append(strategyList, strategyName)
		}
		result[symbol] = strategyList
	}
	return result
}

// ClearCache 清除 ticker 缓存
func (mr *MarketRouter) ClearCache() {
	mr.tickerMu.Lock()
	defer mr.tickerMu.Unlock()
	mr.tickerCache = make(map[perp.Contract]*perp.TickerSnapshot)
	logger.InfoContext(mr.ctx, "Cache cleared")
}

// GetCacheSize 获取 ticker 缓存大小
func (mr *MarketRouter) GetCacheSize() int {
	mr.tickerMu.RLock()
	defer mr.tickerMu.RUnlock()
	return len(mr.tickerCache)
}

// CleanStaleCache 清理过期 ticker 缓存
func (mr *MarketRouter) CleanStaleCache(maxAge time.Duration) int {
	mr.tickerMu.Lock()
	defer mr.tickerMu.Unlock()

	now := time.Now()
	removed := 0
	for symbol, ticker := range mr.tickerCache {
		if ticker == nil {
			delete(mr.tickerCache, symbol)
			removed++
			continue
		}
		if now.Sub(time.UnixMilli(ticker.TimestampMillis)) > maxAge {
			delete(mr.tickerCache, symbol)
			removed++
		}
	}
	if removed > 0 {
		logger.InfoContext(mr.ctx, "Cleaned stale cache entries", logger.Int("removed", removed))
	}
	return removed
}
