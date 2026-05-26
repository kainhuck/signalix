package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kainhuck/signalix/internal/adapters/strategy/pythonipc"
	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/oms"
	"github.com/kainhuck/signalix/internal/app/projection"
	apprisk "github.com/kainhuck/signalix/internal/app/risk"
	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

// Engine 策略引擎 整个服务的核心，负责向上对接交易所行情数据
// 管理策略脚本运行，与策略脚本通信，并收集策略脚本返回的信号
type Engine struct {
	// 策略内存缓存
	strategies map[string]*strategy.Strategy
	strategyMu sync.RWMutex

	// 策略进程
	strategyProcess map[string]strategy.StrategyRuntime
	processMu       sync.RWMutex

	// 依赖
	exchange          ports.Exchange
	loader            *strategy.StrategyLoader
	router            *market.MarketRouter
	decisionEngine    *decision.DecisionEngine
	executionEngine   *oms.ExecutionEngine
	accountProjection *projection.AccountProjection
	riskEvaluator     ports.RiskEvaluator
	store             ports.OrderStore

	// 输出通道
	signalCh chan *models.StrategySignal
	orderCh  chan *models.Order

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 策略状态
	strategyStates  map[string]map[string]interface{} // strategy name -> key -> value
	strategyStateMu sync.RWMutex

	// 订单更新 fan-out：唯一从 OMS orderUpdateCh 读出，再写入 mainOrderCh 与 gRPC 订阅者。
	mainOrderCh      chan *models.Order
	orderStreamSubs  map[uint64]chan *models.Order
	orderSubNext     uint64
	orderSubMu       sync.RWMutex
	orderPumpOnce    sync.Once
	orderPumpWg      sync.WaitGroup
	closeMainOrderCh sync.Once

	// running 为 true 表示 Start 已完全成功（可供 gRPC 等控制面使用）。
	running atomic.Bool

	// defaultInterval 策略 config 未写 interval 时的全局兜底（config.toml）。
	defaultInterval string

	strategyIntervals  map[string]string
	strategyIntervalMu sync.RWMutex

	// 策略自动重启（EH-1）
	restartCfg       config.RestartSettings
	crashTracker     *crashTracker
	restartAttempt   map[string]int
	restartMu        sync.Mutex
	pendingRestart   map[string]time.Time
	pendingRestartMu sync.Mutex
	restartWg        sync.WaitGroup

	equityTracker *apprisk.EquityTracker

	killSwitchMu  sync.RWMutex
	killSwitch    killSwitchState
	killSwitchCfg config.KillSwitchSettings
}

// NewEngine 创建策略引擎；可通过 EngineOption 覆盖风控等默认行为。
func NewEngine(strategyDir string, exchange ports.Exchange, build BuildParams, opts ...EngineOption) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	ch := build.Channels
	signalBuf, orderBuf, mainBuf, marketBuf := ch.Signal, ch.Order, ch.MainOrder, ch.Market
	if signalBuf <= 0 {
		signalBuf = 100
	}
	if orderBuf <= 0 {
		orderBuf = 100
	}
	if mainBuf <= 0 {
		mainBuf = 100
	}
	if marketBuf <= 0 {
		marketBuf = 1000
	}
	divisor := build.DecisionDivisor
	if divisor <= 0 {
		divisor = 10
	}
	omsRetries := build.OMSMaxRetries
	if omsRetries <= 0 {
		omsRetries = 3
	}

	loader := strategy.NewStrategyLoader(ctx, strategyDir)
	equityTracker := apprisk.NewEquityTracker()
	proj := projection.NewAccountProjection(exchange,
		projection.WithRefreshInterval(build.ProjectionRefresh),
		projection.WithEquityHook(equityTracker.OnEquityUpdate),
	)
	router := market.NewMarketRouter(exchange, market.WithBufferSize(marketBuf))

	e := &Engine{
		strategyProcess: make(map[string]strategy.StrategyRuntime),
		exchange:        exchange,
		loader:          loader,
		router:          router,
		defaultInterval: build.DefaultInterval,
		restartCfg:      build.Restart,
		crashTracker:    newCrashTracker(),
		restartAttempt:  make(map[string]int),
		pendingRestart:  make(map[string]time.Time),
		equityTracker:   equityTracker,
		killSwitchCfg:   build.KillSwitch,
		decisionEngine: decision.NewDecisionEngine(proj,
			decision.WithDefaultSizeDivisor(divisor),
			decision.WithExchange(exchange),
			decision.WithTickerLookup(router),
		),
		accountProjection: proj,
		riskEvaluator:     NewStaticRiskEvaluator(risk.DefaultRules()),
		signalCh:          make(chan *models.StrategySignal, signalBuf),
		orderCh:           make(chan *models.Order, orderBuf),
		ctx:               ctx,
		cancel:            cancel,
		strategyStates:    make(map[string]map[string]interface{}),
		mainOrderCh:       make(chan *models.Order, mainBuf),
		orderStreamSubs:   make(map[uint64]chan *models.Order),
	}
	for _, o := range opts {
		o(e)
	}
	omsBuf := ch.OMSOrderUpdate
	if omsBuf <= 0 {
		omsBuf = 100
	}
	cmdBuf := ch.OMSCmd
	if cmdBuf <= 0 {
		cmdBuf = 100
	}
	e.executionEngine = oms.NewExecutionEngine(exchange, proj, e.store,
		oms.WithChannelBuffers(omsBuf, cmdBuf),
		oms.WithMaxRetries(omsRetries),
	)
	return e
}

func (e *Engine) SetAllStrategy(strategies map[string]*strategy.Strategy) {
	e.strategyMu.Lock()
	defer e.strategyMu.Unlock()
	e.strategies = strategies
}

func (e *Engine) SetStrategy(strategy *strategy.Strategy) {
	e.strategyMu.Lock()
	defer e.strategyMu.Unlock()
	e.strategies[strategy.Name] = strategy
}

func (e *Engine) GetStrategy(name string) (*strategy.Strategy, bool) {
	e.strategyMu.RLock()
	defer e.strategyMu.RUnlock()
	s, ok := e.strategies[name]
	return s, ok
}

// SetStrategyCustomState 设置策略自定义状态
func (e *Engine) SetStrategyCustomState(strategyName, key string, value interface{}) {
	e.strategyStateMu.Lock()
	if e.strategyStates[strategyName] == nil {
		e.strategyStates[strategyName] = make(map[string]interface{})
	}
	e.strategyStates[strategyName][key] = value
	e.strategyStateMu.Unlock()

	if e.store == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		logger.ErrorContext(e.ctx, "strategy state marshal failed",
			logger.String("strategy", strategyName),
			logger.String("key", key),
			logger.Any("error", err))
		return
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := e.store.SaveStrategyState(persistCtx, strategyName, key, raw, time.Now()); err != nil {
		logger.ErrorContext(e.ctx, "strategy state persist failed",
			logger.String("strategy", strategyName),
			logger.String("key", key),
			logger.Any("error", err))
	}
}

// GetStrategyCustomState 获取策略自定义状态
func (e *Engine) GetStrategyCustomState(strategyName, key string) (interface{}, bool) {
	e.strategyStateMu.RLock()
	defer e.strategyStateMu.RUnlock()

	if states, ok := e.strategyStates[strategyName]; ok {
		value, exists := states[key]
		return value, exists
	}
	return nil, false
}

// Start 启动策略引擎
func (e *Engine) Start() error {
	// 加载所有的策略到内存中
	if err := e.LoadStrategies(); err != nil {
		return err
	}

	if err := e.loadSnapshotAndReconcile(); err != nil {
		return err
	}

	if err := e.accountProjection.Start(e.ctx); err != nil {
		logger.ErrorContext(e.ctx, "account projection start failed", logger.Any("error", err))
	} else if e.equityTracker != nil {
		if eq, err := e.accountProjection.AccountEquity(); err == nil {
			e.equityTracker.Init(eq, time.Now())
		}
	}

	// 启动所有启用的策略（在 projection 与 OMS 就绪顺序之后，与阶段 6 对账/冷启动一致）
	for _, strategy := range e.strategies {
		if !strategy.Enabled {
			continue
		}
		if err := e.StartStrategy(strategy); err != nil {
			logger.ErrorContext(e.ctx, "failed to start strategy", logger.String("strategy", strategy.Name), logger.Any("error", err))
			continue
		}
	}

	logger.InfoContext(e.ctx, "Strategy Engine started", "strategies", len(e.strategies))

	if err := e.executionEngine.Start(e.ctx); err != nil {
		return err
	}
	e.orderPumpOnce.Do(func() {
		e.orderPumpWg.Add(1)
		go e.orderFanoutLoop()
	})

	// 启动路由器
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.router.Start()
	}()

	// 获取市场路由信息并发送到对应的策略
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.dispatchMarket()
	}()

	// 获取交易信号并交决策引擎，决策引擎将交易信号转为交易订单
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.dispatchSignal()
	}()

	// 风控在 dispatchSignal 中决策之后、写 orderCh 之前执行

	// 执行引擎下单
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.dispatchOrder()
	}()

	e.running.Store(true)
	return nil
}

func (e *Engine) dispatchMarket() {
	for {
		select {
		case marketUpdate := <-e.router.GetMarketChannel():
			switch marketUpdate.Kind {
			case market.MarketUpdateTicker:
				e.dispatchTickerUpdate(marketUpdate)
			case market.MarketUpdateKline:
				e.dispatchKlineUpdate(marketUpdate)
			default:
				if marketUpdate.Ticker != nil {
					marketUpdate.Kind = market.MarketUpdateTicker
					e.dispatchTickerUpdate(marketUpdate)
				}
			}
		case <-e.ctx.Done():
			return
		}
	}
}

func (e *Engine) dispatchTickerUpdate(marketUpdate market.MarketUpdate) {
	traceID := uuid.New().String()
	ticker := marketUpdate.Ticker
	if ticker == nil || ticker.Contract == "" {
		return
	}

	if err := e.SendTick(marketUpdate.StrategyName, models.TickerFromSnapshot(ticker), traceID); err != nil {
		logger.ErrorContext(e.ctx, "failed to send tick",
			logger.String("strategy", marketUpdate.StrategyName),
			logger.Any("error", err),
			logger.String("trace", traceID))
		return
	}

	logger.DebugContext(e.ctx, "send tick success",
		logger.String("strategy", marketUpdate.StrategyName),
		logger.Any("symbol", ticker.Contract),
		logger.String("trace", traceID))
}

func (e *Engine) dispatchKlineUpdate(marketUpdate market.MarketUpdate) {
	snap := marketUpdate.Kline
	if snap == nil || snap.Contract == "" {
		return
	}
	traceID := uuid.New().String()
	bar := models.KlineFromSnapshot(snap)
	if err := e.SendKline(marketUpdate.StrategyName, bar, traceID); err != nil {
		logger.ErrorContext(e.ctx, "failed to send kline",
			logger.String("strategy", marketUpdate.StrategyName),
			logger.Any("error", err),
			logger.String("trace", traceID))
		return
	}
	logger.DebugContext(e.ctx, "send kline success",
		logger.String("strategy", marketUpdate.StrategyName),
		logger.String("contract", string(snap.Contract)),
		logger.String("interval", snap.Interval),
		logger.String("close", snap.Close),
		logger.Int64("timestamp_sec", snap.TimestampSec),
		logger.String("trace", traceID))
}

func (e *Engine) dispatchSignal() {
	for {
		select {
		case signal := <-e.signalCh:
			if e.killSwitchActive() && risk.OpensExposure(signal.Signal) {
				reason := e.KillSwitchStatus().Reason
				logger.WarnContext(e.ctx, "signal rejected by kill switch",
					logger.String("code", "KILL_SWITCH_ACTIVE"),
					logger.String("strategy", signal.StrategyName),
					logger.String("symbol", string(signal.Signal.Symbol)),
					logger.String("direction", string(signal.Signal.Direction)),
					logger.String("kill_switch_reason", reason))
				continue
			}

			order, err := e.decisionEngine.ProcessSignal(e.ctx, signal.StrategyName, signal.Signal)
			if err != nil {
				logger.ErrorContext(e.ctx, "failed to process signal", logger.Any("error", err))
				continue
			}
			if order == nil {
				continue
			}

			riskCtx, err := e.buildRiskContext(e.ctx, signal.StrategyName, signal.Signal, order)
			if err != nil {
				logger.ErrorContext(e.ctx, "failed to build risk context", logger.Any("error", err))
				continue
			}

			stratV, riskCtx, err := e.evaluateStrategyRisk(e.ctx, signal.StrategyName, signal.Signal, order, riskCtx)
			if err != nil {
				logger.ErrorContext(e.ctx, "strategy risk evaluation failed", logger.Any("error", err))
				continue
			}
			switch stratV.Kind {
			case risk.KindReject:
				e.logRiskReject(signal.StrategyName, "strategy", order, stratV)
				continue
			case risk.KindReduce:
				// order.Size 已在 evaluateStrategyRisk 中更新
			case risk.KindAllow:
			default:
				logger.ErrorContext(e.ctx, "unknown strategy risk verdict", logger.String("code", stratV.Code), logger.Any("kind", stratV.Kind))
				continue
			}

			v, err := e.riskEvaluator.Evaluate(e.ctx, riskCtx)
			if err != nil {
				logger.ErrorContext(e.ctx, "risk evaluation failed", logger.Any("error", err))
				continue
			}
			switch v.Kind {
			case risk.KindReject:
				e.logRiskReject(signal.StrategyName, "global", order, v)
				continue
			case risk.KindReduce:
				order.Size = v.AdjustedSize
			case risk.KindAllow:
			default:
				logger.ErrorContext(e.ctx, "unknown risk verdict", logger.String("code", v.Code), logger.Any("kind", v.Kind))
				continue
			}

			select {
			case e.orderCh <- order:
			case <-e.ctx.Done():
				return
			}

		case <-e.ctx.Done():
			return
		}
	}
}

func (e *Engine) dispatchOrder() {
	for {
		select {
		case order := <-e.orderCh:
			err := e.executionEngine.SubmitOrder(e.ctx, order)
			if err != nil {
				logger.ErrorContext(e.ctx, "failed to submit order", logger.Any("error", err))
				continue
			}

			logger.InfoContext(e.ctx, "submit order success", logger.String("strategy", order.StrategyName), logger.String("id", order.ID))

		case <-e.ctx.Done():
			return
		}
	}
}

// Stop 停止策略引擎
func (e *Engine) Stop() error {
	logger.InfoContext(e.ctx, "Stopping Strategy Engine...")

	e.running.Store(false)

	e.router.Stop()

	// 停止所有策略
	if err := e.StopAllStrategies(); err != nil {
		logger.ErrorContext(e.ctx, "failed to stop all strategies", logger.Any("error", err))
	}

	if e.store != nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 20*time.Second)
		if err := e.executionEngine.FlushOrdersToStore(flushCtx); err != nil {
			logger.ErrorContext(e.ctx, "flush orders to store failed", logger.Any("error", err))
		}
		if err := e.flushStrategyStatesToStore(flushCtx); err != nil {
			logger.ErrorContext(e.ctx, "flush strategy states failed", logger.Any("error", err))
		}
		flushCancel()
	}

	// 取消 context
	e.cancel()

	e.restartWg.Wait()

	// 等待所有 goroutine 结束
	e.wg.Wait()

	if e.accountProjection != nil {
		e.accountProjection.Stop()
	}
	if err := e.executionEngine.Stop(); err != nil {
		logger.ErrorContext(e.ctx, "execution engine stop", logger.Any("error", err))
	}

	e.orderPumpWg.Wait()
	e.closeMainOrderCh.Do(func() {
		close(e.mainOrderCh)
	})

	// 关闭通道
	close(e.signalCh)

	if e.store != nil {
		if err := e.store.Close(); err != nil {
			logger.ErrorContext(context.Background(), "order store close", logger.Any("error", err))
		}
		e.store = nil
	}

	logger.InfoContext(e.ctx, "Strategy Engine stopped")
	return nil
}

// StartStrategy 启动策略（手动路径：重置自动重启退避计数）。
func (e *Engine) StartStrategy(st *strategy.Strategy) error {
	e.resetRestartAttempt(st.Name)
	return e.launchStrategy(st)
}

// launchStrategy 启动或重启策略进程（不重置退避计数，供自动重启使用）。
func (e *Engine) launchStrategy(st *strategy.Strategy) error {
	e.processMu.Lock()
	defer e.processMu.Unlock()

	interval, err := strategy.ResolveInterval(st.StrategyConfig, e.defaultInterval)
	if err != nil {
		return fmt.Errorf("strategy %q: %w", st.Name, err)
	}
	if err := e.router.Subscribe(st.Name, st.Symbols, interval, st.SubscribeTicker); err != nil {
		return fmt.Errorf("failed to subscribe strategy %q: %v", st.Name, err)
	}

	// 检查策略是否已经在运行
	if _, exists := e.strategyProcess[st.Name]; exists {
		return nil
	}

	// 创建策略进程
	sp, err := pythonipc.NewStrategyProcess(e.ctx, st)
	if err != nil {
		return fmt.Errorf("failed to create strategy process: %w", err)
	}

	// 保存到 map
	e.strategyProcess[st.Name] = sp

	// 启动消息读取 goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.readStrategyMessages(sp)
	}()

	// 启动 stderr 读取 goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		sp.ReadStderr()
	}()

	// 启动心跳监控 goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.monitorHeartbeat(sp)
	}()

	// 发送初始化消息
	if err := sp.SendInit(st); err != nil {
		return fmt.Errorf("failed to send init message: %w", err)
	}

	if err := e.warmupHistory(st, sp, interval); err != nil {
		return fmt.Errorf("failed to send history for strategy %q: %w", st.Name, err)
	}

	e.setStrategyInterval(st.Name, interval)

	logger.InfoContext(e.ctx, "Strategy started", logger.String("strategy", st.Name))
	return nil
}

// warmupHistory 拉取 REST 历史 K 线并发送 history IPC。
func (e *Engine) warmupHistory(st *strategy.Strategy, sp strategy.StrategyRuntime, interval string) error {
	if st.HistoryBars <= 0 {
		return nil
	}

	series := make([]*models.KlineSeries, 0, len(st.Symbols))
	for _, sym := range st.Symbols {
		snaps, err := e.exchange.ListCandlesticks(e.ctx, &perp.ListCandlesticksQuery{
			Contract: sym,
			Interval: interval,
			Limit:    st.HistoryBars,
		})
		if err != nil {
			logger.ErrorContext(e.ctx, "list candlesticks for history failed",
				logger.String("strategy", st.Name),
				logger.String("contract", string(sym)),
				logger.String("interval", interval),
				logger.Any("error", err))
			continue
		}
		bars := models.KlinesFromSnapshots(snaps)
		if len(bars) == 0 {
			continue
		}
		series = append(series, &models.KlineSeries{
			Contract: sym,
			Bars:     bars,
		})
	}

	if len(series) == 0 {
		logger.WarnContext(e.ctx, "history warmup skipped: no bars",
			logger.String("strategy", st.Name))
		return nil
	}

	payload := &models.HistoryPayload{
		Interval: interval,
		Series:   series,
	}
	e.router.IngestHistoryKlines(interval, series)

	if err := sp.SendHistory(payload); err != nil {
		return err
	}

	barCount := 0
	for _, s := range series {
		barCount += len(s.Bars)
	}
	logger.InfoContext(e.ctx, "send history success",
		logger.String("strategy", st.Name),
		logger.String("interval", interval),
		logger.Int("series", len(series)),
		logger.Int("bars", barCount))
	return nil
}

// StopStrategy 停止策略
func (e *Engine) StopStrategy(name string) error {
	e.processMu.Lock()
	sp, exists := e.strategyProcess[name]
	if !exists {
		e.processMu.Unlock()
		return fmt.Errorf("strategy not found: %s", name)
	}
	delete(e.strategyProcess, name)

	_ = e.router.UnsubscribeAll(name)
	e.removeStrategyInterval(name)

	e.processMu.Unlock()

	return sp.Stop()
}

// StartStrategyByName 按 catalog 名称启动策略（已在运行时幂等成功）。
func (e *Engine) StartStrategyByName(name string) error {
	s, ok := e.GetStrategy(name)
	if !ok || s == nil {
		return fmt.Errorf("strategy not found: %s", name)
	}
	return e.StartStrategy(s)
}

// ReloadStrategiesCatalog 重新 Discover 并刷新内存 catalog，不启停已运行进程。
func (e *Engine) ReloadStrategiesCatalog() (int, error) {
	if err := e.LoadStrategies(); err != nil {
		return 0, err
	}
	e.strategyMu.RLock()
	n := len(e.strategies)
	e.strategyMu.RUnlock()
	return n, nil
}

// GetStrategiesDir 获取策略目录
func (e *Engine) GetStrategiesDir() string {
	if e.loader == nil {
		return ""
	}
	return e.loader.StrategiesDir
}

// SendTick 发送 ticker 到策略
func (e *Engine) SendTick(strategyName string, ticker *models.Ticker, traceID string) error {
	e.processMu.RLock()
	sp, exists := e.strategyProcess[strategyName]
	e.processMu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy not found: %s", strategyName)
	}

	return sp.SendTick(ticker, traceID)
}

// SendKline 发送收盘 K 线到策略
func (e *Engine) SendKline(strategyName string, kline *models.Kline, traceID string) error {
	e.processMu.RLock()
	sp, exists := e.strategyProcess[strategyName]
	e.processMu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy not found: %s", strategyName)
	}

	return sp.SendKline(kline, traceID)
}

// readStrategyMessages 读取策略消息
func (e *Engine) readStrategyMessages(sp strategy.StrategyRuntime) {
	err := sp.ReadMessages(func(msg strategy.IpcMessage) {
		e.handleStrategyMessage(sp, msg)
	})

	if err != nil {
		logger.ErrorContext(e.ctx, "Read messages failed", logger.String("strategy", sp.Name()), logger.Any("error", err))
	}

	// 进程退出
	e.handleProcessExit(sp)
}

// handleStrategyMessage 处理策略消息
func (e *Engine) handleStrategyMessage(sp strategy.StrategyRuntime, msg strategy.IpcMessage) {
	switch msg.Type {
	case "heartbeat":
		// 更新心跳时间
		sp.UpdateHeartbeat()

	case "init_ack":
		// 初始化确认
		if err, ok := msg.Data["error"]; ok {
			logger.ErrorContext(e.ctx, "Strategy init_ack failed", logger.String("strategy", sp.Name()), logger.Any("error", err))
		} else {
			logger.InfoContext(e.ctx, "Strategy init_ack checked", logger.String("strategy", sp.Name()))
		}

	case "signal":
		logger.InfoContext(e.ctx, "Receive Strategy signal", logger.String("strategy", sp.Name()), logger.Any("data", msg.Data))
		// 处理交易信号
		var signal models.Signal
		data, _ := json.Marshal(msg.Data)
		if err := json.Unmarshal(data, &signal); err != nil {
			logger.ErrorContext(e.ctx, "Failed to parse signal", logger.String("strategy", sp.Name()), logger.Any("error", err))
			return
		}

		// 发送到信号通道
		select {
		case e.signalCh <- &models.StrategySignal{
			StrategyName: sp.Name(),
			Signal:       &signal,
			Timestamp:    time.Now(),
		}:
		case <-e.ctx.Done():
		}

	case "rpc_request":
		// 处理 RPC 请求
		go e.handleRPCRequest(sp, msg)

	case "log":
		// 处理日志
		level, _ := msg.Data["level"].(string)
		message, _ := msg.Data["message"].(string)
		logger.DebugContext(e.ctx, "[strategy log]", logger.String("strategy", sp.Name()), logger.String("level", level), logger.String("message", message))

		// TODO 保存到数据库

	default:
		logger.WarnContext(e.ctx, "Unknown message type", logger.String("strategy", sp.Name()), logger.String("type", msg.Type))
	}
}

// handleRPCRequest 处理 RPC 请求
func (e *Engine) handleRPCRequest(sp strategy.StrategyRuntime, msg strategy.IpcMessage) {
	requestID, _ := msg.Data["request_id"].(string)
	method, _ := msg.Data["method"].(string)
	params, _ := msg.Data["params"].(map[string]interface{})
	//ctx := context.WithValue(sp.Context(), "request_id", requestID)

	var result interface{}
	var err error

	switch method {
	case "get_position":
		symbol, _ := params["symbol"].(string)
		result, err = e.rpcGetPosition(symbol)

	case "get_balance":
		result, err = e.rpcGetBalance()

	case "get_market":
		err = fmt.Errorf("method not implemented: get_market")

	case "get_klines":
		result, err = e.rpcGetKlines(sp.Name(), params)

	case "get_markets":
		// TODO
		//symbols, _ := params["symbols"].([]interface{})
		//symbolStrs := make([]string, len(symbols))
		//for i, s := range symbols {
		//	symbolStrs[i], _ = s.(string)
		//}
		//result, err = e.stateProvider.GetMarkets(symbolStrs)

	case "get_state":
		key, _ := params["key"].(string)
		state, ok := e.GetStrategyCustomState(sp.Name(), key)
		result = state
		if !ok {
			err = fmt.Errorf("can't find state for %s", key)
		}

	case "set_state":
		key, _ := params["key"].(string)
		value := params["value"]
		e.SetStrategyCustomState(sp.Name(), key, value)
		result = map[string]string{"status": "ok"}

	default:
		err = fmt.Errorf("unknown method: %s", method)
	}

	// 发送响应
	if err := sp.SendRPCResponse(requestID, result, err); err != nil {
		logger.ErrorContext(e.ctx, "Failed to send RPC response",
			logger.String("strategy", sp.Name()),
			logger.Any("error", err),
			logger.String("request_id", requestID))
	}
}

// monitorHeartbeat 监控心跳
func (e *Engine) monitorHeartbeat(sp strategy.StrategyRuntime) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-sp.Context().Done():
			return
		case <-ticker.C:
			lastHeartbeat := sp.GetLastHeartbeat()

			// 稳定运行：窗口内无崩溃则清零退避计数
			if e.crashCountInWindow(sp.Name()) == 0 {
				e.resetRestartAttempt(sp.Name())
			}

			// 检查心跳超时（30秒）
			if time.Since(lastHeartbeat) > 30*time.Second {
				logger.ErrorContext(e.ctx, "Heartbeat timeout, marking as crashed", logger.String("strategy", sp.Name()))
				e.handleProcessExit(sp)
				return
			}
		}
	}
}

// handleProcessExit 处理进程退出
func (e *Engine) handleProcessExit(sp strategy.StrategyRuntime) {
	name := sp.Name()

	e.processMu.Lock()
	_, stillRegistered := e.strategyProcess[name]
	if stillRegistered {
		delete(e.strategyProcess, name)
		_ = e.router.UnsubscribeAll(name)
	}
	e.processMu.Unlock()

	if !stillRegistered {
		return
	}

	crashCount := e.recordStrategyCrash(name)

	logger.ErrorContext(e.ctx, "Strategy process exited",
		logger.String("strategy", name),
		logger.Int("crash_count", crashCount),
		logger.Duration("window", e.restartCfg.CrashWindow))

	if e.ctx.Err() != nil {
		return
	}
	if !e.restartCfg.Enabled {
		return
	}
	if e.killSwitchActive() {
		return
	}
	if crashCount >= e.restartCfg.MaxCrashes {
		logger.ErrorContext(e.ctx, "Strategy restart circuit open, not restarting",
			logger.String("strategy", name),
			logger.Int("crash_count", crashCount),
			logger.Int("max_crashes", e.restartCfg.MaxCrashes))
		return
	}

	st, ok := e.GetStrategy(name)
	if !ok || st == nil || !st.Enabled {
		return
	}

	e.scheduleRestart(name, st)
}

// ListStrategies 列出所有运行中的策略
func (e *Engine) ListStrategies() []string {
	e.processMu.RLock()
	defer e.processMu.RUnlock()

	names := make([]string, 0, len(e.strategyProcess))
	for name := range e.strategyProcess {
		names = append(names, name)
	}
	return names
}

// GetStrategyStatus 获取策略状态（catalog + 运行时；未运行进程时 running=false）。
func (e *Engine) GetStrategyStatus(name string) (map[string]interface{}, error) {
	snap, err := e.StrategyRuntimeSnapshot(name)
	if err != nil {
		return nil, err
	}
	return strategyRuntimeSnapshotToMap(snap), nil
}

// LoadStrategies 加载所有策略
func (e *Engine) LoadStrategies() error {
	if e.loader == nil {
		return fmt.Errorf("strategy loader not initialized, call SetStrategiesDir first")
	}

	strategies, err := e.loader.DiscoverStrategies()
	if err != nil {
		return fmt.Errorf("failed to load strategies: %w", err)
	}

	logger.InfoContext(e.ctx, "Found strategies", logger.Int("count", len(strategies)), logger.String("dir", e.GetStrategiesDir()))

	e.SetAllStrategy(strategies)

	return nil
}

// StopAllStrategies 停止所有策略
func (e *Engine) StopAllStrategies() error {
	e.processMu.RLock()
	names := make([]string, 0, len(e.strategyProcess))
	for name := range e.strategyProcess {
		names = append(names, name)
	}
	e.processMu.RUnlock()

	for _, name := range names {
		if err := e.StopStrategy(name); err != nil {
			logger.ErrorContext(e.ctx, "Failed to stop strategy", logger.String("strategy", name), logger.Any("error", err))
		}
	}

	return nil
}

func (e *Engine) flushStrategyStatesToStore(ctx context.Context) error {
	if e.store == nil {
		return nil
	}
	e.strategyStateMu.RLock()
	defer e.strategyStateMu.RUnlock()
	for sn, m := range e.strategyStates {
		for k, v := range m {
			raw, err := json.Marshal(v)
			if err != nil {
				logger.ErrorContext(ctx, "flush strategy state marshal failed",
					logger.String("strategy", sn),
					logger.String("key", k),
					logger.Any("error", err))
				continue
			}
			if err := e.store.SaveStrategyState(ctx, sn, k, raw, time.Now()); err != nil {
				return fmt.Errorf("save strategy state %s/%s: %w", sn, k, err)
			}
		}
	}
	return nil
}

func (e *Engine) orderFanoutLoop() {
	defer e.orderPumpWg.Done()
	for o := range e.executionEngine.OrderUpdateChannel() {
		if o == nil {
			continue
		}
		select {
		case e.mainOrderCh <- o:
		default:
			logger.WarnContext(e.ctx, "main order fanout channel full",
				logger.String("order_id", o.ID))
		}
		e.orderSubMu.RLock()
		for id, ch := range e.orderStreamSubs {
			select {
			case ch <- o:
			default:
				logger.WarnContext(e.ctx, "order grpc subscriber channel full",
					logger.Uint64("subscriber_id", id),
					logger.String("order_id", o.ID))
			}
		}
		e.orderSubMu.RUnlock()
	}
}

// Running 表示 Engine.Start 已完整成功且尚未进入 Stop。
func (e *Engine) Running() bool {
	return e.running.Load()
}

// ListStrategyCatalog 返回 Discover 后的策略元数据（含未运行条目），按名称排序。
func (e *Engine) ListStrategyCatalog() []*strategy.Strategy {
	e.strategyMu.RLock()
	defer e.strategyMu.RUnlock()
	out := make([]*strategy.Strategy, 0, len(e.strategies))
	for _, s := range e.strategies {
		if s != nil {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RegisterOrderSubscriber 注册订单更新接收端；ctx 取消时注销并关闭返回的 channel。
func (e *Engine) RegisterOrderSubscriber(ctx context.Context, buf int) <-chan *models.Order {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan *models.Order, buf)
	id := atomic.AddUint64(&e.orderSubNext, 1)
	e.orderSubMu.Lock()
	e.orderStreamSubs[id] = ch
	e.orderSubMu.Unlock()
	go func() {
		<-ctx.Done()
		e.orderSubMu.Lock()
		delete(e.orderStreamSubs, id)
		close(ch)
		e.orderSubMu.Unlock()
	}()
	return ch
}

// GetOrderSnapshot 读 OMS 中的订单快照。
func (e *Engine) GetOrderSnapshot(orderID string) (*models.Order, bool) {
	return e.executionEngine.GetOrder(orderID)
}

// ListOpenOrdersSnapshot 列出非终态订单，最多 limit 条（<=0 时用 500）。
func (e *Engine) ListOpenOrdersSnapshot(limit int) []*models.Order {
	if limit <= 0 {
		limit = 500
	}
	var out []*models.Order
	for _, o := range e.executionEngine.GetAllOrders() {
		if o == nil {
			continue
		}
		switch o.Status {
		case models.OrderStatusFilled, models.OrderStatusCancelled, models.OrderStatusRejected:
			continue
		default:
			out = append(out, o)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

// CancelOrderViaOMS 撤单（经 OMS 队列）。
func (e *Engine) CancelOrderViaOMS(ctx context.Context, orderID string) error {
	return e.executionEngine.CancelOrder(ctx, orderID)
}

// BuildInfoVersion 返回主模块版本摘要（用于 gRPC GetEngineInfo）。
func (e *Engine) BuildInfoVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func (e *Engine) GetOrderChan() <-chan *models.Order {
	return e.mainOrderCh
}
