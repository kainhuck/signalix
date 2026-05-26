package oms

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

// ExecutionEngine 执行引擎（OMS：单 goroutine 串行处理提交、撤单、同步与用户流更新）。
type ExecutionEngine struct {
	exchange ports.Exchange
	acctProj *projection.AccountProjection
	store    ports.OrderStore
	orders   map[string]*models.Order // order ID -> order
	mu       sync.RWMutex

	orderUpdateCh chan *models.Order
	cmdCh         chan *omsCmd

	// runCtx 由 Start 注入，与引擎生命周期一致；用于 sendOrderUpdate 在关停时可退出。
	runCtx context.Context

	runWg    sync.WaitGroup
	stopOnce sync.Once

	// started 在 run goroutine 启动后置 1，避免在未 Start 时向 cmdCh 投递 flush 导致死锁。
	started int32

	maxRetries    int
	retryInterval time.Duration

	stats ExecutionStats
}

// ExecutionStats 执行统计
type ExecutionStats struct {
	SubmittedOrders int64
	FilledOrders    int64
	CancelledOrders int64
	RejectedOrders  int64
	FailedOrders    int64
	Retries         int64
	LastUpdateTime  time.Time
	mu              sync.RWMutex
}

// NewExecutionEngine 创建执行引擎；projection 可为 nil（仅测试），生产路径应注入 AccountProjection。
func NewExecutionEngine(exchange ports.Exchange, acctProj *projection.AccountProjection, store ports.OrderStore, opts ...Option) *ExecutionEngine {
	e := &ExecutionEngine{
		exchange:      exchange,
		acctProj:      acctProj,
		store:         store,
		orders:        make(map[string]*models.Order),
		orderUpdateCh: make(chan *models.Order, 100),
		cmdCh:         make(chan *omsCmd, 100),
		maxRetries:    3,
		retryInterval: time.Second * 2,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Start 启动 OMS 循环（须与上层 Engine 使用同一 ctx）。
func (e *ExecutionEngine) Start(ctx context.Context) error {
	e.runCtx = ctx
	atomic.StoreInt32(&e.started, 1)
	e.runWg.Add(1)
	go func() {
		defer e.runWg.Done()
		e.run(ctx)
	}()
	return nil
}

// Stop 在 ctx 已取消且 run 退出之后调用：关闭 orderUpdateCh。
func (e *ExecutionEngine) Stop() error {
	e.stopOnce.Do(func() {
		e.runWg.Wait()
		close(e.orderUpdateCh)
	})
	return nil
}

func (e *ExecutionEngine) run(ctx context.Context) {
	userCh := e.exchange.UserEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-e.cmdCh:
			if !ok {
				return
			}
			if cmd == nil {
				continue
			}
			switch cmd.op {
			case omsOpSubmit:
				e.execSubmit(ctx, cmd)
			case omsOpCancel:
				e.execCancel(cmd)
			case omsOpSync:
				e.execSync(cmd)
			case omsOpFlush:
				e.execFlush(cmd)
			}
		case ev, ok := <-userCh:
			if !ok {
				return
			}
			if e.acctProj != nil {
				e.acctProj.OnUserEvent(ctx, ev)
			}
			e.dispatchUserEvent(ev)
		}
	}
}

func (e *ExecutionEngine) execSubmit(ctx context.Context, cmd *omsCmd) {
	order := cmd.order
	if order == nil {
		cmd.reply <- fmt.Errorf("nil order")
		return
	}
	if cmd.reply == nil {
		return
	}

	e.mu.Lock()
	if existing, ok := e.orders[order.ID]; ok {
		e.mu.Unlock()
		cmd.reply <- fmt.Errorf("duplicate order id %q: existing order in state %s", order.ID, existing.Status)
		return
	}
	e.orders[order.ID] = order
	order.Status = models.OrderStatusPending
	now := time.Now()
	order.CreatedAt = now
	order.UpdatedAt = now
	e.mu.Unlock()

	e.sendOrderUpdate(order)

	e.stats.mu.Lock()
	e.stats.SubmittedOrders++
	e.stats.LastUpdateTime = time.Now()
	e.stats.mu.Unlock()

	cmd.reply <- e.submitWithRetry(ctx, order)
}

func (e *ExecutionEngine) execCancel(cmd *omsCmd) {
	if cmd.reply == nil {
		return
	}

	e.mu.RLock()
	order, exists := e.orders[cmd.orderID]
	if !exists {
		e.mu.RUnlock()
		cmd.reply <- fmt.Errorf("order not found: %s", cmd.orderID)
		return
	}
	st := order.Status
	e.mu.RUnlock()

	if st == models.OrderStatusFilled ||
		st == models.OrderStatusCancelled ||
		st == models.OrderStatusRejected {
		cmd.reply <- fmt.Errorf("cannot cancel order in status: %s", st)
		return
	}

	if err := e.exchange.Cancel(cmd.ctx, &perp.CancelParams{
		Contract: order.Symbol,
		OrderID:  order.ExchangeID,
	}); err != nil {
		cmd.reply <- fmt.Errorf("failed to cancel order: %w", err)
		return
	}

	e.mu.Lock()
	if o, ok := e.orders[cmd.orderID]; ok {
		o.Status = models.OrderStatusCancelled
		o.UpdatedAt = time.Now()
	}
	e.mu.Unlock()

	e.sendOrderUpdate(order)

	e.stats.mu.Lock()
	e.stats.CancelledOrders++
	e.stats.mu.Unlock()

	cmd.reply <- nil
}

func (e *ExecutionEngine) execSync(cmd *omsCmd) {
	if cmd.reply == nil {
		return
	}
	e.mu.RLock()
	order, exists := e.orders[cmd.orderID]
	e.mu.RUnlock()
	if !exists {
		cmd.reply <- fmt.Errorf("order not found: %s", cmd.orderID)
		return
	}

	exchangeOrder, err := e.exchange.GetOrder(cmd.ctx, order.Symbol, order.ExchangeID)
	if err != nil {
		cmd.reply <- fmt.Errorf("failed to get order from exchange: %w", err)
		return
	}

	e.mu.Lock()
	order.Status = models.OrderStatus(exchangeOrder.Status)
	order.FilledSize = exchangeOrder.FilledSize
	order.UpdatedAt = exchangeOrder.UpdatedAt
	e.mu.Unlock()

	e.sendOrderUpdate(order)

	cmd.reply <- nil
}

func (e *ExecutionEngine) execFlush(cmd *omsCmd) {
	if cmd.reply == nil {
		return
	}
	if e.store == nil {
		cmd.reply <- nil
		return
	}
	ctx := context.Background()
	if cmd.ctx != nil {
		ctx = cmd.ctx
	}
	e.mu.RLock()
	list := make([]*models.Order, 0, len(e.orders))
	for _, o := range e.orders {
		list = append(list, o)
	}
	e.mu.RUnlock()
	for _, o := range list {
		if o == nil {
			continue
		}
		if err := e.store.SaveOrder(ctx, cloneOrder(o)); err != nil {
			cmd.reply <- err
			return
		}
	}
	cmd.reply <- nil
}

// SubmitOrder 将订单投递到 OMS 队列（不在调用方 goroutine 内 Place）。
func (e *ExecutionEngine) SubmitOrder(ctx context.Context, order *models.Order) error {
	if order == nil {
		return fmt.Errorf("nil order")
	}
	reply := make(chan error, 1)
	c := &omsCmd{op: omsOpSubmit, order: order, reply: reply}
	select {
	case e.cmdCh <- c:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// submitWithRetry 仅在 OMS run goroutine 内调用。
func (e *ExecutionEngine) submitWithRetry(ctx context.Context, order *models.Order) error {
	var lastErr error

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			e.handleSubmitError(order, err)
			return err
		default:
		}

		if attempt > 0 {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				e.handleSubmitError(order, err)
				return err
			case <-time.After(e.retryInterval):
			}

			e.stats.mu.Lock()
			e.stats.Retries++
			e.stats.mu.Unlock()
		}

		req := e.buildOrderRequest(order)
		resp, err := e.exchange.Place(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}

		e.handleSubmitSuccess(order, resp)
		return nil
	}

	err := fmt.Errorf("failed after %d retries: %w", e.maxRetries, lastErr)
	e.handleSubmitError(order, err)
	return err
}

func (e *ExecutionEngine) buildOrderRequest(order *models.Order) *perp.PlaceRequest {
	// ClientID 与本地 Order.ID 一致，作为所侧幂等键（Gate text 等）；重试 Place 复用同一 ClientID。
	return &perp.PlaceRequest{
		Contract:    order.Symbol,
		Side:        perp.Side(order.Side),
		Type:        perp.OrderType(order.OrderType),
		Size:        order.Size,
		Price:       order.Price,
		TimeInForce: perp.TIFGTC,
		ReduceOnly:  false,
		ClientID:    order.ID,
	}
}

func (e *ExecutionEngine) handleSubmitSuccess(order *models.Order, resp *perp.OrderSnapshot) {
	e.mu.Lock()
	order.ExchangeID = resp.ExchangeOrderID
	order.Status = models.OrderStatusSubmitted
	order.UpdatedAt = time.Now()
	e.mu.Unlock()

	e.sendOrderUpdate(order)
}

func (e *ExecutionEngine) handleSubmitError(order *models.Order, err error) {
	e.mu.Lock()
	order.Status = models.OrderStatusRejected
	order.UpdatedAt = time.Now()
	e.mu.Unlock()

	e.sendOrderUpdate(order)

	e.stats.mu.Lock()
	e.stats.RejectedOrders++
	e.stats.FailedOrders++
	e.stats.mu.Unlock()
}

// CancelOrder 撤单（OMS 串行执行）。
func (e *ExecutionEngine) CancelOrder(ctx context.Context, orderID string) error {
	reply := make(chan error, 1)
	c := &omsCmd{op: omsOpCancel, orderID: orderID, ctx: ctx, reply: reply}
	select {
	case e.cmdCh <- c:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetOrder 获取订单（读路径 RLock）。
func (e *ExecutionEngine) GetOrder(orderID string) (*models.Order, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	order, exists := e.orders[orderID]
	return order, exists
}

// GetAllOrders 获取所有订单
func (e *ExecutionEngine) GetAllOrders() []*models.Order {
	e.mu.RLock()
	defer e.mu.RUnlock()

	orders := make([]*models.Order, 0, len(e.orders))
	for _, order := range e.orders {
		orders = append(orders, order)
	}
	return orders
}

// GetOrdersBySymbol 获取指定 symbol 的订单
func (e *ExecutionEngine) GetOrdersBySymbol(symbol perp.Contract) []*models.Order {
	e.mu.RLock()
	defer e.mu.RUnlock()

	orders := make([]*models.Order, 0)
	for _, order := range e.orders {
		if order.Symbol == symbol {
			orders = append(orders, order)
		}
	}
	return orders
}

// GetOrdersByStatus 获取指定状态的订单
func (e *ExecutionEngine) GetOrdersByStatus(status models.OrderStatus) []*models.Order {
	e.mu.RLock()
	defer e.mu.RUnlock()

	orders := make([]*models.Order, 0)
	for _, order := range e.orders {
		if order.Status == status {
			orders = append(orders, order)
		}
	}
	return orders
}

// NonFinalOrderCount 统计未终态本地订单数（Pending / Submitted / PartialFilled）。
func (e *ExecutionEngine) NonFinalOrderCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.nonFinalOrderCountLocked("")
}

// NonFinalOrderCountForStrategy 统计指定策略的非终态订单数。
func (e *ExecutionEngine) NonFinalOrderCountForStrategy(strategyName string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.nonFinalOrderCountLocked(strategyName)
}

func (e *ExecutionEngine) nonFinalOrderCountLocked(strategyName string) int {
	n := 0
	for _, o := range e.orders {
		if o == nil {
			continue
		}
		if strategyName != "" && o.StrategyName != strategyName {
			continue
		}
		switch o.Status {
		case models.OrderStatusPending, models.OrderStatusSubmitted, models.OrderStatusPartialFilled:
			n++
		}
	}
	return n
}

// HydrateFromSnapshot 在 Start 之前将快照订单灌入内存（单线程调用；不得与 run 并发）。
func (e *ExecutionEngine) HydrateFromSnapshot(orders []*models.Order) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, o := range orders {
		if o == nil {
			continue
		}
		if c := cloneOrder(o); c != nil {
			e.orders[c.ID] = c
		}
	}
}

// FlushOrdersToStore 将当前内存订单全量刷盘（须在 OMS run 仍存活且 ctx 未取消时调用）。
func (e *ExecutionEngine) FlushOrdersToStore(ctx context.Context) error {
	if atomic.LoadInt32(&e.started) == 0 {
		return nil
	}
	if e.store == nil {
		return nil
	}
	reply := make(chan error, 1)
	c := &omsCmd{op: omsOpFlush, ctx: ctx, reply: reply}
	select {
	case e.cmdCh <- c:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// dispatchUserEvent 处理用户流订单更新（仅在 OMS goroutine 内调用）。
func (e *ExecutionEngine) dispatchUserEvent(update *perp.UserEvent) {
	if update == nil || update.Kind != perp.UserOrderUpdate {
		return
	}
	ov, ok := update.Order()
	if !ok {
		return
	}

	e.mu.Lock()
	var order *models.Order
	for _, o := range e.orders {
		if o.ExchangeID == ov.ExchangeOrderID {
			order = o
			break
		}
	}
	if order == nil {
		e.mu.Unlock()
		return
	}

	order.Status = models.OrderStatus(ov.Status)
	order.FilledSize = ov.FilledSize
	order.UpdatedAt = ov.UpdatedAt
	filled := order.Status == models.OrderStatusFilled
	e.mu.Unlock()

	e.sendOrderUpdate(order)

	if filled {
		e.stats.mu.Lock()
		e.stats.FilledOrders++
		e.stats.mu.Unlock()
	}
}

// sendOrderUpdate 发送订单更新（正常路径阻塞发送；关停时若 ctx 已取消则不再无限阻塞）。
func (e *ExecutionEngine) sendOrderUpdate(order *models.Order) {
	if e.runCtx == nil {
		select {
		case e.orderUpdateCh <- order:
		default:
			return
		}
		e.syncOrderToStore(order)
		return
	}
	select {
	case e.orderUpdateCh <- order:
	case <-e.runCtx.Done():
		return
	}
	e.syncOrderToStore(order)
}

func cloneOrder(o *models.Order) *models.Order {
	if o == nil {
		return nil
	}
	c := *o
	if o.Price != nil {
		p := *o.Price
		c.Price = &p
	}
	if o.StopPrice != nil {
		p := *o.StopPrice
		c.StopPrice = &p
	}
	return &c
}

func (e *ExecutionEngine) syncOrderToStore(order *models.Order) {
	if e.store == nil || order == nil {
		return
	}
	ctx := context.Background()
	if e.runCtx != nil {
		ctx = e.runCtx
	}
	if err := e.store.SaveOrder(ctx, cloneOrder(order)); err != nil {
		logger.ErrorContext(ctx, "oms persist order failed",
			logger.String("order_id", order.ID),
			logger.Any("error", err))
	}
}

// OrderUpdateChannel 获取订单更新通道
func (e *ExecutionEngine) OrderUpdateChannel() <-chan *models.Order {
	return e.orderUpdateCh
}

// GetStats 获取统计信息
func (e *ExecutionEngine) GetStats() ExecutionStats {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()

	return ExecutionStats{
		SubmittedOrders: e.stats.SubmittedOrders,
		FilledOrders:    e.stats.FilledOrders,
		CancelledOrders: e.stats.CancelledOrders,
		RejectedOrders:  e.stats.RejectedOrders,
		FailedOrders:    e.stats.FailedOrders,
		Retries:         e.stats.Retries,
		LastUpdateTime:  e.stats.LastUpdateTime,
	}
}

// SetMaxRetries 设置最大重试次数
func (e *ExecutionEngine) SetMaxRetries(maxRetries int) {
	e.maxRetries = maxRetries
}

// SetRetryInterval 设置重试间隔
func (e *ExecutionEngine) SetRetryInterval(interval time.Duration) {
	e.retryInterval = interval
}

// SyncOrderStatus 同步订单状态（OMS 串行执行）。
func (e *ExecutionEngine) SyncOrderStatus(ctx context.Context, orderID string) error {
	reply := make(chan error, 1)
	c := &omsCmd{op: omsOpSync, orderID: orderID, ctx: ctx, reply: reply}
	select {
	case e.cmdCh <- c:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
