package market

import (
	"context"
	"fmt"
	"sync"

	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

const perpOrderEventBuf = 64

// PerpExecutorConfig perp MarketExecutor 依赖。
type PerpExecutorConfig struct {
	Exchange ports.Exchange
	Proj     *projection.AccountProjection
}

// PerpExecutor 实现 perp 的 MarketExecutor（含用户流泵与 projection 更新）。
type PerpExecutor struct {
	exchange    ports.Exchange
	proj        *projection.AccountProjection
	orderEvents chan *models.OrderEvent

	mu      sync.Mutex
	runCtx  context.Context
	cancel  context.CancelFunc
	pumpWg  sync.WaitGroup
	started bool
}

// NewPerpExecutor 构造 perp 执行器。
func NewPerpExecutor(cfg PerpExecutorConfig) *PerpExecutor {
	return &PerpExecutor{
		exchange:    cfg.Exchange,
		proj:        cfg.Proj,
		orderEvents: make(chan *models.OrderEvent, perpOrderEventBuf),
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

// Place 满足 MarketExecutor。
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
	return resp.ExchangeOrderID, nil
}

// Cancel 满足 MarketExecutor。
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

// Sync 满足 MarketExecutor。
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

// OrderEvents 满足 MarketExecutor。
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
			oe := orderEventFromUserEvent(ev)
			if oe == nil {
				continue
			}
			select {
			case p.orderEvents <- oe:
			case <-ctx.Done():
				return
			}
		}
	}
}

func orderEventFromUserEvent(ev *perp.UserEvent) *models.OrderEvent {
	if ev == nil || ev.Kind != perp.UserOrderUpdate {
		return nil
	}
	ov, ok := ev.Order()
	if !ok || ov == nil {
		return nil
	}
	return &models.OrderEvent{
		Market:     models.MarketPerp,
		ExchangeID: ov.ExchangeOrderID,
		Status:     models.OrderStatus(ov.Status),
		FilledSize: ov.FilledSize,
		UpdatedAt:  ov.UpdatedAt,
	}
}

var _ MarketExecutor = (*PerpExecutor)(nil)
