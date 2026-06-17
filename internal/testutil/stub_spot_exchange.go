package testutil

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

// StubSpotExchange 测试用现货交易所替身，实现 ports.SpotExchange。
type StubSpotExchange struct {
	Mu sync.Mutex

	PairMetas        []*spot.PairMeta
	BalancesList     []*spot.BalanceView
	CandlesticksList []*spot.CandlestickSnapshot
	Placed           []*spot.PlaceRequest
	Orders           map[string]*spot.OrderSnapshot

	PingErr         error
	ListPairMetaErr error
	BalancesErr     error
	ListCandlesErr  error
	PlaceErr        error
	CancelErr       error
	GetOrderErr     error
	SubscribeErr    error
	UnsubscribeErr  error
	ConnectErr      error
	PublicCh        chan *spot.PublicEvent
	UserCh          chan *spot.UserEvent
	Subscribed      []*spot.Subscription
	Unsubscribed    []*spot.Subscription
}

var _ ports.SpotExchange = (*StubSpotExchange)(nil)

func NewStubSpotExchange() *StubSpotExchange {
	return &StubSpotExchange{
		PairMetas: []*spot.PairMeta{{
			Pair:            "BTC/USDT",
			MinBaseAmount:   "0.0001",
			MinQuoteAmount:  "1",
			AmountPrecision: 8,
			PricePrecision:  2,
			TradeStatus:     "tradable",
		}},
		BalancesList: []*spot.BalanceView{{
			Currency:  "USDT",
			Total:     "10000",
			Available: "10000",
			Frozen:    "0",
			UpdatedAt: time.Now(),
		}},
		Orders:   make(map[string]*spot.OrderSnapshot),
		PublicCh: make(chan *spot.PublicEvent, 16),
		UserCh:   make(chan *spot.UserEvent, 16),
	}
}

func (s *StubSpotExchange) Ping(ctx context.Context) error {
	_ = ctx
	return s.PingErr
}

func (s *StubSpotExchange) Close(ctx context.Context) error {
	_ = ctx
	return nil
}

func (s *StubSpotExchange) Connect(ctx context.Context, parts spot.ConnectParts) error {
	_ = ctx
	_ = parts
	return s.ConnectErr
}

func (s *StubSpotExchange) ListPairMeta(ctx context.Context) ([]*spot.PairMeta, error) {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.ListPairMetaErr != nil {
		return nil, s.ListPairMetaErr
	}
	out := make([]*spot.PairMeta, 0, len(s.PairMetas))
	for _, m := range s.PairMetas {
		if m == nil {
			continue
		}
		cp := *m
		cp.Pair = cp.Pair.Canonical()
		out = append(out, &cp)
	}
	return out, nil
}

func (s *StubSpotExchange) ListCandlesticks(ctx context.Context, q *spot.ListCandlesticksQuery) ([]*spot.CandlestickSnapshot, error) {
	_ = ctx
	_ = q
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.ListCandlesErr != nil {
		return nil, s.ListCandlesErr
	}
	out := make([]*spot.CandlestickSnapshot, 0, len(s.CandlesticksList))
	for _, c := range s.CandlesticksList {
		if c == nil {
			continue
		}
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

func (s *StubSpotExchange) Subscribe(ctx context.Context, subs []*spot.Subscription) error {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.SubscribeErr != nil {
		return s.SubscribeErr
	}
	s.Subscribed = append(s.Subscribed, cloneSpotSubscriptions(subs)...)
	return nil
}

func (s *StubSpotExchange) Unsubscribe(ctx context.Context, subs []*spot.Subscription) error {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.UnsubscribeErr != nil {
		return s.UnsubscribeErr
	}
	s.Unsubscribed = append(s.Unsubscribed, cloneSpotSubscriptions(subs)...)
	return nil
}

func (s *StubSpotExchange) PublicEvents() <-chan *spot.PublicEvent {
	return s.PublicCh
}

func (s *StubSpotExchange) UserEvents() <-chan *spot.UserEvent {
	return s.UserCh
}

func (s *StubSpotExchange) Place(ctx context.Context, req *spot.PlaceRequest) (*spot.OrderSnapshot, error) {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.PlaceErr != nil {
		return nil, s.PlaceErr
	}
	cp := *req
	s.Placed = append(s.Placed, &cp)
	id := strings.TrimSpace(req.ClientID)
	if id == "" {
		id = "spot-local-1"
	}
	order := &spot.OrderSnapshot{
		OrderID:         id,
		ExchangeOrderID: "spot-ex-1",
		ClientID:        req.ClientID,
		Pair:            req.Pair,
		Side:            req.Side,
		Type:            req.Type,
		Size:            req.Size,
		Status:          spot.OrderPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if req.QuoteAmount != nil {
		order.Size = *req.QuoteAmount
	}
	s.Orders[id] = order
	return cloneSpotOrder(order), nil
}

func (s *StubSpotExchange) Cancel(ctx context.Context, p *spot.CancelParams) error {
	_ = ctx
	_ = p
	return s.CancelErr
}

func (s *StubSpotExchange) GetOrder(ctx context.Context, pair spot.Pair, orderID string) (*spot.OrderSnapshot, error) {
	_ = ctx
	_ = pair
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.GetOrderErr != nil {
		return nil, s.GetOrderErr
	}
	if s.Orders != nil {
		if o := s.Orders[orderID]; o != nil {
			return cloneSpotOrder(o), nil
		}
	}
	return &spot.OrderSnapshot{
		OrderID:         orderID,
		ExchangeOrderID: orderID,
		Pair:            pair.Canonical(),
		Status:          spot.OrderPending,
		UpdatedAt:       time.Now(),
	}, nil
}

func (s *StubSpotExchange) Balances(ctx context.Context) ([]*spot.BalanceView, error) {
	_ = ctx
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.BalancesErr != nil {
		return nil, s.BalancesErr
	}
	out := make([]*spot.BalanceView, 0, len(s.BalancesList))
	for _, b := range s.BalancesList {
		if b == nil {
			continue
		}
		cp := *b
		cp.Currency = strings.ToUpper(strings.TrimSpace(cp.Currency))
		out = append(out, &cp)
	}
	return out, nil
}

func (s *StubSpotExchange) Balance(ctx context.Context, currency string) (*spot.BalanceView, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	list, err := s.Balances(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range list {
		if b.Currency == currency {
			return b, nil
		}
	}
	return &spot.BalanceView{Currency: currency, Total: "0", Available: "0", Frozen: "0"}, nil
}

func cloneSpotOrder(o *spot.OrderSnapshot) *spot.OrderSnapshot {
	if o == nil {
		return nil
	}
	cp := *o
	return &cp
}

func cloneSpotSubscriptions(in []*spot.Subscription) []*spot.Subscription {
	out := make([]*spot.Subscription, 0, len(in))
	for _, s := range in {
		if s == nil {
			continue
		}
		cp := *s
		cp.Pairs = append([]spot.Pair(nil), s.Pairs...)
		cp.Payload = append([]string(nil), s.Payload...)
		out = append(out, &cp)
	}
	return out
}
