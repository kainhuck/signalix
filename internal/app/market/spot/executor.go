package spot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/kainhuck/signalix/pkg/logger"
)

const spotOrderEventBuf = 100

type SpotExecutor struct {
	exchange    ports.SpotExchange
	orderEvents chan *models.OrderEvent

	mu      sync.Mutex
	runCtx  context.Context
	cancel  context.CancelFunc
	pumpWg  sync.WaitGroup
	started bool
}

func NewSpotExecutor(exchange ports.SpotExchange) *SpotExecutor {
	return &SpotExecutor{
		exchange:    exchange,
		orderEvents: make(chan *models.OrderEvent, spotOrderEventBuf),
	}
}

func (e *SpotExecutor) Start(ctx context.Context) error {
	if e == nil || e.exchange == nil {
		return fmt.Errorf("spot executor not configured")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.runCtx, e.cancel = context.WithCancel(ctx)
	e.started = true
	e.pumpWg.Add(1)
	go func() {
		defer e.pumpWg.Done()
		e.pump(e.runCtx)
	}()
	return nil
}

func (e *SpotExecutor) Stop() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return nil
	}
	cancel := e.cancel
	e.started = false
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.pumpWg.Wait()
	return nil
}

func (e *SpotExecutor) Place(ctx context.Context, o *models.Order) (string, error) {
	if e == nil || e.exchange == nil {
		return "", fmt.Errorf("spot executor not configured")
	}
	if o == nil {
		return "", fmt.Errorf("nil order")
	}

	pair := spot.CanonicalPair(string(o.Symbol))
	req := &spot.PlaceRequest{
		Pair:        pair,
		Side:        spot.Side(o.Side),
		Type:        spot.OrderType(o.OrderType),
		Size:        o.Size,
		Price:       o.Price,
		TimeInForce: spot.TIFGTC,
		ClientID:    o.ID,
	}

	if o.OrderType == models.OrderTypeMarket && o.Side == models.OrderSideBuy {
		req.QuoteAmount = strPtr(o.Size)
		req.Size = ""
	}

	resp, err := e.exchange.Place(ctx, req)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("nil place response")
	}
	return resp.ExchangeOrderID, nil
}

func (e *SpotExecutor) Cancel(ctx context.Context, o *models.Order) error {
	if e == nil || e.exchange == nil {
		return fmt.Errorf("spot executor not configured")
	}
	if o == nil {
		return fmt.Errorf("nil order")
	}
	return e.exchange.Cancel(ctx, &spot.CancelParams{
		Pair:    spot.CanonicalPair(string(o.Symbol)),
		OrderID: o.ExchangeID,
	})
}

func (e *SpotExecutor) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	if e == nil || e.exchange == nil {
		return nil, fmt.Errorf("spot executor not configured")
	}
	if o == nil {
		return nil, fmt.Errorf("nil order")
	}
	pair := spot.CanonicalPair(string(o.Symbol))
	snap, err := e.exchange.GetOrder(ctx, pair, o.ExchangeID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, fmt.Errorf("nil order snapshot")
	}
	return &models.OrderEvent{
		Market:     models.MarketSpot,
		ExchangeID: snap.ExchangeOrderID,
		ClientID:   o.ID,
		Status:     models.OrderStatus(snap.Status),
		FilledSize: snap.FilledSize,
		UpdatedAt:  snap.UpdatedAt,
	}, nil
}

func (e *SpotExecutor) OrderEvents() <-chan *models.OrderEvent {
	if e == nil {
		ch := make(chan *models.OrderEvent)
		return ch
	}
	return e.orderEvents
}

func (e *SpotExecutor) pump(ctx context.Context) {
	userCh := e.exchange.UserEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-userCh:
			if !ok {
				return
			}
			oe := e.orderEventFromUserEvent(ev)
			if oe == nil {
				continue
			}
			e.emitOrderEvent(ctx, oe)
		}
	}
}

func (e *SpotExecutor) orderEventFromUserEvent(ev *spot.UserEvent) *models.OrderEvent {
	if e == nil || ev == nil || ev.Kind != spot.UserOrderUpdate {
		return nil
	}
	ov, ok := ev.Order()
	if !ok || ov == nil {
		return nil
	}
	return &models.OrderEvent{
		Market:     models.MarketSpot,
		ExchangeID: ov.ExchangeOrderID,
		Status:     models.OrderStatus(ov.Status),
		FilledSize: ov.FilledSize,
		UpdatedAt:  ov.UpdatedAt,
	}
}

func (e *SpotExecutor) emitOrderEvent(ctx context.Context, oe *models.OrderEvent) {
	select {
	case e.orderEvents <- oe:
	case <-ctx.Done():
	default:
		logger.WarnContext(ctx, "Channel full, dropping spot order event",
			logger.String("exchange_id", oe.ExchangeID),
			logger.Any("status", oe.Status))
	}
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

var _ market.MarketExecutor = (*SpotExecutor)(nil)
