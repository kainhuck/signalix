package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// StubExchange 测试用交易所替身，实现 ports.Exchange（perp.Live）。
type StubExchange struct {
	Mu sync.Mutex

	BalanceAvailable string
	PositionSnapshot *perp.PositionSnapshot
	PositionErr      error
	PositionsList    []*perp.PositionSnapshot
	ContractMetas    []*perp.ContractMeta

	// GetOrderHook 若非 nil 则替代默认 GetOrder（默认可返回 ORDER_NOT_FOUND）。
	GetOrderHook func(ctx context.Context, contract perp.Contract, orderID string) (*perp.OrderSnapshot, error)
}

var _ ports.Exchange = (*StubExchange)(nil)

// NewStubExchange 返回默认余额与无持仓的替身。
func NewStubExchange() *StubExchange {
	return &StubExchange{
		BalanceAvailable: "10000",
		PositionErr:      perp.NewError(perp.ErrPositionNotFound, "stub no position", nil),
	}
}

func (s *StubExchange) Ping(ctx context.Context) error { return nil }

func (s *StubExchange) Close(ctx context.Context) error { return nil }

func (s *StubExchange) ListContractMeta(ctx context.Context) ([]*perp.ContractMeta, error) {
	s.Mu.Lock()
	list := s.ContractMetas
	s.Mu.Unlock()
	if len(list) == 0 {
		return nil, nil
	}
	out := make([]*perp.ContractMeta, 0, len(list))
	for _, m := range list {
		if m == nil {
			continue
		}
		cp := *m
		out = append(out, &cp)
	}
	return out, nil
}

func (s *StubExchange) ListCandlesticks(ctx context.Context, q *perp.ListCandlesticksQuery) ([]*perp.CandlestickSnapshot, error) {
	return nil, nil
}

func (s *StubExchange) Subscribe(ctx context.Context, subs []*perp.Subscription) error { return nil }

func (s *StubExchange) Unsubscribe(ctx context.Context, subs []*perp.Subscription) error { return nil }

func (s *StubExchange) PublicEvents() <-chan *perp.PublicEvent {
	ch := make(chan *perp.PublicEvent)
	return ch
}

func (s *StubExchange) Place(ctx context.Context, req *perp.PlaceRequest) (*perp.OrderSnapshot, error) {
	return &perp.OrderSnapshot{
		ExchangeOrderID: "stub-ex-1",
		Contract:        req.Contract,
		Status:          perp.OrderPending,
		UpdatedAt:       time.Now(),
	}, nil
}

func (s *StubExchange) Cancel(ctx context.Context, p *perp.CancelParams) error { return nil }

func (s *StubExchange) GetOrder(ctx context.Context, contract perp.Contract, orderID string) (*perp.OrderSnapshot, error) {
	if s.GetOrderHook != nil {
		return s.GetOrderHook(ctx, contract, orderID)
	}
	return nil, perp.NewError(perp.ErrOrderNotFound, "stub", nil)
}

func (s *StubExchange) Positions(ctx context.Context) ([]*perp.PositionSnapshot, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if len(s.PositionsList) > 0 {
		out := make([]*perp.PositionSnapshot, 0, len(s.PositionsList))
		for _, pv := range s.PositionsList {
			if pv == nil {
				continue
			}
			c := *pv
			out = append(out, &c)
		}
		return out, nil
	}
	return nil, nil
}

func (s *StubExchange) Position(ctx context.Context, contract perp.Contract) (*perp.PositionSnapshot, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.PositionErr != nil {
		return nil, s.PositionErr
	}
	return s.PositionSnapshot, nil
}

func (s *StubExchange) Balance(ctx context.Context) (*perp.BalanceView, error) {
	s.Mu.Lock()
	avail := s.BalanceAvailable
	s.Mu.Unlock()
	if avail == "" {
		avail = "0"
	}
	return &perp.BalanceView{
		Currency:  "USDT",
		Total:     avail,
		Available: avail,
		Frozen:    "0",
		UpdatedAt: time.Now(),
	}, nil
}

func (s *StubExchange) UserEvents() <-chan *perp.UserEvent {
	ch := make(chan *perp.UserEvent)
	return ch
}

func (s *StubExchange) Connect(ctx context.Context, parts perp.ConnectParts) error { return nil }
