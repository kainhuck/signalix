package spot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/kainhuck/signalix/pkg/logger"
)

const spotOrderEventBuf = 64

type SpotExecutorConfig struct {
	Exchange   ports.SpotExchange
	Projection *AccountProjection
}

type SpotExecutor struct {
	exchange    ports.SpotExchange
	proj        *AccountProjection
	orderEvents chan *models.OrderEvent

	mu      sync.Mutex
	runCtx  context.Context
	cancel  context.CancelFunc
	pumpWg  sync.WaitGroup
	started bool
}

func NewSpotExecutor(cfg SpotExecutorConfig) (*SpotExecutor, error) {
	if cfg.Exchange == nil {
		return nil, fmt.Errorf("exchange is required")
	}
	return &SpotExecutor{
		exchange:    cfg.Exchange,
		proj:        cfg.Projection,
		orderEvents: make(chan *models.OrderEvent, spotOrderEventBuf),
	}, nil
}

func (s *SpotExecutor) Start(ctx context.Context) error {
	if s == nil || s.exchange == nil {
		return fmt.Errorf("spot executor not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.runCtx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.pumpWg.Add(1)
	go func() {
		defer s.pumpWg.Done()
		s.pump(s.runCtx)
	}()
	return nil
}

func (s *SpotExecutor) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.started = false
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.pumpWg.Wait()
	return nil
}

func (s *SpotExecutor) Place(ctx context.Context, o *models.Order) (string, error) {
	if s == nil || s.exchange == nil {
		return "", fmt.Errorf("spot executor not configured")
	}
	req, err := placeRequestFromOrder(o)
	if err != nil {
		return "", err
	}
	resp, err := s.exchange.Place(ctx, req)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("nil place response")
	}
	if id := strings.TrimSpace(resp.ExchangeOrderID); id != "" {
		return id, nil
	}
	if id := strings.TrimSpace(resp.OrderID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("empty exchange order id")
}

func (s *SpotExecutor) Cancel(ctx context.Context, o *models.Order) error {
	if s == nil || s.exchange == nil {
		return fmt.Errorf("spot executor not configured")
	}
	if o == nil {
		return fmt.Errorf("nil order")
	}
	pair := spotex.CanonicalPair(string(o.Symbol))
	if pair == "" {
		return fmt.Errorf("symbol is required")
	}
	return s.exchange.Cancel(ctx, &spotex.CancelParams{
		Pair:          pair,
		OrderID:       o.ExchangeID,
		ClientOrderID: o.ID,
	})
}

func (s *SpotExecutor) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	if s == nil || s.exchange == nil {
		return nil, fmt.Errorf("spot executor not configured")
	}
	if o == nil {
		return nil, fmt.Errorf("nil order")
	}
	pair := spotex.CanonicalPair(string(o.Symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	snap, err := s.exchange.GetOrder(ctx, pair, o.ExchangeID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, fmt.Errorf("nil order snapshot")
	}
	return orderEventFromSnapshot(snap, o.ID), nil
}

func (s *SpotExecutor) OrderEvents() <-chan *models.OrderEvent {
	if s == nil {
		ch := make(chan *models.OrderEvent)
		return ch
	}
	return s.orderEvents
}

func (s *SpotExecutor) pump(ctx context.Context) {
	userCh := s.exchange.UserEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-userCh:
			if !ok {
				return
			}
			if s.proj != nil {
				s.proj.OnUserEvent(ev)
			}
			snap, ok := ev.Order()
			if !ok || snap == nil {
				continue
			}
			s.emitOrderEvent(ctx, orderEventFromSnapshot(snap, snap.ClientID))
		}
	}
}

func (s *SpotExecutor) emitOrderEvent(ctx context.Context, oe *models.OrderEvent) {
	if oe == nil {
		return
	}
	select {
	case s.orderEvents <- oe:
	case <-ctx.Done():
	default:
		logger.WarnContext(ctx, "Channel full, dropping spot order event",
			logger.String("exchange_id", oe.ExchangeID),
			logger.Any("status", oe.Status))
	}
}

func placeRequestFromOrder(o *models.Order) (*spotex.PlaceRequest, error) {
	if o == nil {
		return nil, fmt.Errorf("nil order")
	}
	if o.Market != "" && o.Market != models.MarketSpot {
		return nil, fmt.Errorf("order market %q is not spot", o.Market)
	}
	pair := spotex.CanonicalPair(string(o.Symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	req := &spotex.PlaceRequest{
		Pair:        pair,
		Side:        spotex.Side(o.Side),
		Type:        spotex.OrderType(o.OrderType),
		Price:       o.Price,
		TimeInForce: spotex.TIFGTC,
		ClientID:    o.ID,
		Account:     "spot",
	}
	switch req.Side {
	case spotex.SideBuy, spotex.SideSell:
	default:
		return nil, fmt.Errorf("invalid side: %s", o.Side)
	}
	switch req.Type {
	case spotex.OrderTypeMarket:
		if req.Side == spotex.SideBuy {
			size := strings.TrimSpace(o.Size)
			req.QuoteAmount = &size
		} else {
			req.Size = strings.TrimSpace(o.Size)
		}
	case spotex.OrderTypeLimit:
		req.Size = strings.TrimSpace(o.Size)
		if req.Price == nil || strings.TrimSpace(*req.Price) == "" {
			return nil, fmt.Errorf("limit order requires price")
		}
	default:
		return nil, fmt.Errorf("unsupported order type: %s", o.OrderType)
	}
	return req, nil
}

func orderEventFromSnapshot(snap *spotex.OrderSnapshot, clientID string) *models.OrderEvent {
	if snap == nil {
		return nil
	}
	exchangeID := strings.TrimSpace(snap.ExchangeOrderID)
	if exchangeID == "" {
		exchangeID = strings.TrimSpace(snap.OrderID)
	}
	if clientID == "" {
		clientID = strings.TrimSpace(snap.ClientID)
	}
	return &models.OrderEvent{
		Market:     models.MarketSpot,
		ExchangeID: exchangeID,
		ClientID:   clientID,
		Status:     models.OrderStatus(snap.Status),
		FilledSize: snap.FilledSize,
		UpdatedAt:  snap.UpdatedAt,
	}
}

var _ market.MarketExecutor = (*SpotExecutor)(nil)
