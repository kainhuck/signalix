package spot

import "fmt"

// PublicPayload 公共 WS 事件载荷。
type PublicPayload interface {
	isPublicPayload()
}

// UserPayload 私有 WS 事件载荷。
type UserPayload interface {
	isUserPayload()
}

func (*TickerSnapshot) isPublicPayload()      {}
func (*CandlestickSnapshot) isPublicPayload() {}
func (*OrderSnapshot) isUserPayload()         {}
func (*BalanceUpdateSnapshot) isUserPayload() {}

// NewPublicEvent 构造公共事件。
func NewPublicEvent(kind PublicKind, payload PublicPayload) (*PublicEvent, error) {
	if payload == nil {
		return nil, fmt.Errorf("spot: public event payload is nil")
	}
	if err := validatePublicKindPayload(kind, payload); err != nil {
		return nil, err
	}
	return &PublicEvent{Kind: kind, Payload: payload}, nil
}

// NewUserEvent 构造私有事件。
func NewUserEvent(kind UserKind, payload UserPayload) (*UserEvent, error) {
	if payload == nil {
		return nil, fmt.Errorf("spot: user event payload is nil")
	}
	if err := validateUserKindPayload(kind, payload); err != nil {
		return nil, err
	}
	return &UserEvent{Kind: kind, Payload: payload}, nil
}

func validatePublicKindPayload(kind PublicKind, payload PublicPayload) error {
	switch kind {
	case PublicTicker:
		if _, ok := payload.(*TickerSnapshot); !ok {
			return fmt.Errorf("spot: kind %s requires *TickerSnapshot", kind)
		}
	case PublicCandlestick:
		if _, ok := payload.(*CandlestickSnapshot); !ok {
			return fmt.Errorf("spot: kind %s requires *CandlestickSnapshot", kind)
		}
	default:
		return fmt.Errorf("spot: unknown public kind %s", kind)
	}
	return nil
}

func validateUserKindPayload(kind UserKind, payload UserPayload) error {
	switch kind {
	case UserOrderUpdate:
		if _, ok := payload.(*OrderSnapshot); !ok {
			return fmt.Errorf("spot: kind %s requires *OrderSnapshot", kind)
		}
	case UserBalanceUpdate:
		if _, ok := payload.(*BalanceUpdateSnapshot); !ok {
			return fmt.Errorf("spot: kind %s requires *BalanceUpdateSnapshot", kind)
		}
	default:
		return fmt.Errorf("spot: unknown user kind %s", kind)
	}
	return nil
}

// Ticker 在 Kind 为 PublicTicker 时返回 ticker 载荷。
func (e *PublicEvent) Ticker() (*TickerSnapshot, bool) {
	if e == nil || e.Kind != PublicTicker {
		return nil, false
	}
	t, ok := e.Payload.(*TickerSnapshot)
	return t, ok
}

// Candlestick 在 Kind 为 PublicCandlestick 时返回 K 线载荷。
func (e *PublicEvent) Candlestick() (*CandlestickSnapshot, bool) {
	if e == nil || e.Kind != PublicCandlestick {
		return nil, false
	}
	c, ok := e.Payload.(*CandlestickSnapshot)
	return c, ok
}

// Order 在 Kind 为 UserOrderUpdate 时返回订单载荷。
func (e *UserEvent) Order() (*OrderSnapshot, bool) {
	if e == nil || e.Kind != UserOrderUpdate {
		return nil, false
	}
	o, ok := e.Payload.(*OrderSnapshot)
	return o, ok
}

// Balance 在 Kind 为 UserBalanceUpdate 时返回余额变更载荷。
func (e *UserEvent) Balance() (*BalanceUpdateSnapshot, bool) {
	if e == nil || e.Kind != UserBalanceUpdate {
		return nil, false
	}
	b, ok := e.Payload.(*BalanceUpdateSnapshot)
	return b, ok
}
