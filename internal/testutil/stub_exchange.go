package testutil

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// StubExchange 测试用交易所替身，实现 ports.PerpExchange（perp.Live）。
type StubExchange struct {
	Mu sync.Mutex

	BalanceAvailable string
	BalanceErr       error
	PositionsErr     error
	PositionSnapshot *perp.PositionSnapshot
	PositionErr      error
	PositionsList    []*perp.PositionSnapshot
	ContractMetas    []*perp.ContractMeta
	PlaceCalls       int

	// GetOrderHook 若非 nil 则替代默认 GetOrder（默认可返回 ORDER_NOT_FOUND）。
	GetOrderHook func(ctx context.Context, contract perp.Contract, orderID string) (*perp.OrderSnapshot, error)
}

var _ ports.PerpExchange = (*StubExchange)(nil)

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
	s.Mu.Lock()
	s.PlaceCalls++
	s.Mu.Unlock()
	return &perp.OrderSnapshot{
		ExchangeOrderID: "stub-ex-1",
		ClientID:        s.TagFromLocal(req.ClientID),
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
	if s.PositionsErr != nil {
		return nil, s.PositionsErr
	}
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
	err := s.BalanceErr
	s.Mu.Unlock()
	if err != nil {
		return nil, err
	}
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

const stubClientTagPrefix = "stub:"

// TagFromLocal 满足 perp.ClientOrderIDCodec（测试替身）。
func (s *StubExchange) TagFromLocal(localID string) string {
	localID = strings.TrimSpace(localID)
	if localID == "" {
		return ""
	}
	return stubClientTagPrefix + localID
}

// LocalFromTag 满足 perp.ClientOrderIDCodec。
func (s *StubExchange) LocalFromTag(tag string) (string, bool) {
	tag = strings.TrimSpace(tag)
	if !strings.HasPrefix(tag, stubClientTagPrefix) {
		return "", false
	}
	local := strings.TrimPrefix(tag, stubClientTagPrefix)
	if local == "" {
		return "", false
	}
	return local, true
}
