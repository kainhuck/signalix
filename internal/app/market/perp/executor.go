package perp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

const perpOrderEventBuf = 64

// PerpExecutorConfig perp market.MarketExecutor 依赖。
type PerpExecutorConfig struct {
	Exchange ports.Exchange
	Proj     *projection.AccountProjection
}

// PerpExecutor 实现 perp 的 market.MarketExecutor（含用户流泵与 projection 更新）。
type PerpExecutor struct {
	exchange    ports.Exchange
	proj        *projection.AccountProjection
	orderEvents chan *models.OrderEvent

	gateTextMu      sync.Mutex
	gateTextToLocal map[string]string

	mu      sync.Mutex
	runCtx  context.Context
	cancel  context.CancelFunc
	pumpWg  sync.WaitGroup
	started bool
}

// NewPerpExecutor 构造 perp 执行器。
func NewPerpExecutor(cfg PerpExecutorConfig) *PerpExecutor {
	return &PerpExecutor{
		exchange:        cfg.Exchange,
		proj:            cfg.Proj,
		orderEvents:     make(chan *models.OrderEvent, perpOrderEventBuf),
		gateTextToLocal: make(map[string]string),
	}
}

// Start 启动用户流泵（须在 Exchange 可用之后调用）。
func (p *PerpExecutor) Start(ctx context.Context) error {
	if p == nil || p.exchange == nil {
		return fmt.Errorf("perp executor not configured")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil
	}
	p.runCtx, p.cancel = context.WithCancel(ctx)
	p.started = true
	p.pumpWg.Add(1)
	go func() {
		defer p.pumpWg.Done()
		p.pump(p.runCtx)
	}()
	return nil
}

// Stop 停止用户流泵。
func (p *PerpExecutor) Stop() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}
	cancel := p.cancel
	p.started = false
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	p.pumpWg.Wait()
	return nil
}

// Place 满足 market.MarketExecutor。
func (p *PerpExecutor) Place(ctx context.Context, o *models.Order) (string, error) {
	if p == nil || p.exchange == nil {
		return "", fmt.Errorf("perp executor not configured")
	}
	if o == nil {
		return "", fmt.Errorf("nil order")
	}
	req := &perp.PlaceRequest{
		Contract:    o.Symbol,
		Side:        perp.Side(o.Side),
		Type:        perp.OrderType(o.OrderType),
		Size:        o.Size,
		Price:       o.Price,
		TimeInForce: perp.TIFGTC,
		ReduceOnly:  false,
		ClientID:    o.ID,
	}
	resp, err := p.exchange.Place(ctx, req)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("nil place response")
	}
	p.registerGateText(o.ID)
	return resp.ExchangeOrderID, nil
}

// Cancel 满足 market.MarketExecutor。
func (p *PerpExecutor) Cancel(ctx context.Context, o *models.Order) error {
	if p == nil || p.exchange == nil {
		return fmt.Errorf("perp executor not configured")
	}
	if o == nil {
		return fmt.Errorf("nil order")
	}
	return p.exchange.Cancel(ctx, &perp.CancelParams{
		Contract: o.Symbol,
		OrderID:  o.ExchangeID,
	})
}

// Sync 满足 market.MarketExecutor。
func (p *PerpExecutor) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	if p == nil || p.exchange == nil {
		return nil, fmt.Errorf("perp executor not configured")
	}
	if o == nil {
		return nil, fmt.Errorf("nil order")
	}
	snap, err := p.exchange.GetOrder(ctx, o.Symbol, o.ExchangeID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, fmt.Errorf("nil order snapshot")
	}
	clientID := o.ID
	return &models.OrderEvent{
		Market:     models.MarketPerp,
		ExchangeID: snap.ExchangeOrderID,
		ClientID:   clientID,
		Status:     models.OrderStatus(snap.Status),
		FilledSize: snap.FilledSize,
		UpdatedAt:  snap.UpdatedAt,
	}, nil
}

// OrderEvents 满足 market.MarketExecutor。
func (p *PerpExecutor) OrderEvents() <-chan *models.OrderEvent {
	if p == nil {
		ch := make(chan *models.OrderEvent)
		return ch
	}
	return p.orderEvents
}

func (p *PerpExecutor) pump(ctx context.Context) {
	userCh := p.exchange.UserEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-userCh:
			if !ok {
				return
			}
			if p.proj != nil {
				p.proj.OnUserEvent(ctx, ev)
			}
			oe, gateText := p.orderEventFromUserEvent(ev)
			if oe == nil {
				continue
			}
			p.emitOrderEvent(ctx, oe)
			if gateText != "" && isTerminalOrderStatus(oe.Status) {
				p.unregisterGateText(gateText)
			}
		}
	}
}

func (p *PerpExecutor) registerGateText(localID string) {
	if p == nil || localID == "" {
		return
	}
	gateText := perp.GateTextFromClientOrderID(localID)
	if gateText == "" {
		return
	}
	p.gateTextMu.Lock()
	p.gateTextToLocal[gateText] = localID
	p.gateTextMu.Unlock()
}

func (p *PerpExecutor) unregisterGateText(gateText string) {
	gateText = strings.TrimSpace(gateText)
	if p == nil || gateText == "" {
		return
	}
	p.gateTextMu.Lock()
	delete(p.gateTextToLocal, gateText)
	p.gateTextMu.Unlock()
}

func (p *PerpExecutor) resolveLocalClientID(gateText string) string {
	gateText = strings.TrimSpace(gateText)
	if gateText == "" {
		return ""
	}
	p.gateTextMu.Lock()
	local, ok := p.gateTextToLocal[gateText]
	p.gateTextMu.Unlock()
	if ok {
		return local
	}
	return perp.LocalClientIDFromGateText(gateText)
}

func isTerminalOrderStatus(st models.OrderStatus) bool {
	switch st {
	case models.OrderStatusFilled, models.OrderStatusCancelled, models.OrderStatusRejected:
		return true
	default:
		return false
	}
}

func (p *PerpExecutor) emitOrderEvent(ctx context.Context, oe *models.OrderEvent) {
	select {
	case p.orderEvents <- oe:
	case <-ctx.Done():
	default:
		logger.WarnContext(ctx, "Channel full, dropping order event",
			logger.String("exchange_id", oe.ExchangeID),
			logger.Any("status", oe.Status))
	}
}

func (p *PerpExecutor) orderEventFromUserEvent(ev *perp.UserEvent) (*models.OrderEvent, string) {
	if p == nil || ev == nil || ev.Kind != perp.UserOrderUpdate {
		return nil, ""
	}
	ov, ok := ev.Order()
	if !ok || ov == nil {
		return nil, ""
	}
	gateText := strings.TrimSpace(ov.ClientID)
	return &models.OrderEvent{
		Market:     models.MarketPerp,
		ExchangeID: ov.ExchangeOrderID,
		ClientID:   p.resolveLocalClientID(gateText),
		Status:     models.OrderStatus(ov.Status),
		FilledSize: ov.FilledSize,
		UpdatedAt:  ov.UpdatedAt,
	}, gateText
}

var _ market.MarketExecutor = (*PerpExecutor)(nil)
