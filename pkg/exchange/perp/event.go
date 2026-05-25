package perp

import "fmt"

// PublicPayload 公共 WS 事件载荷（仅 perp 包内可实现）。
type PublicPayload interface {
	isPublicPayload()
}

// UserPayload 私有 WS 事件载荷（仅 perp 包内可实现）。
type UserPayload interface {
	isUserPayload()
}

func (*TickerSnapshot) isPublicPayload()      {}
func (*CandlestickSnapshot) isPublicPayload() {}
func (*OrderSnapshot) isUserPayload()         {}
func (*BalanceUpdateSnapshot) isUserPayload() {}
func (*TradeSnapshot) isUserPayload()         {}
func (*PositionSnapshot) isUserPayload()      {}

// NewPublicEvent 构造公共事件；校验 Kind 与 Payload 类型一致。
func NewPublicEvent(kind PublicKind, payload PublicPayload) (*PublicEvent, error) {
	if payload == nil {
		return nil, fmt.Errorf("perp: public event payload is nil")
	}
	if err := validatePublicKindPayload(kind, payload); err != nil {
		return nil, err
	}
	return &PublicEvent{Kind: kind, Payload: payload}, nil
}

// NewUserEvent 构造私有事件；校验 Kind 与 Payload 类型一致。
func NewUserEvent(kind UserKind, payload UserPayload) (*UserEvent, error) {
	if payload == nil {
		return nil, fmt.Errorf("perp: user event payload is nil")
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
			return fmt.Errorf("perp: kind %s requires *TickerSnapshot", kind)
		}
	case PublicCandlestick:
		if _, ok := payload.(*CandlestickSnapshot); !ok {
			return fmt.Errorf("perp: kind %s requires *CandlestickSnapshot", kind)
		}
	default:
		return fmt.Errorf("perp: unknown public kind %s", kind)
	}
	return nil
}

func validateUserKindPayload(kind UserKind, payload UserPayload) error {
	switch kind {
	case UserOrderUpdate:
		if _, ok := payload.(*OrderSnapshot); !ok {
			return fmt.Errorf("perp: kind %s requires *OrderSnapshot", kind)
		}
	case UserBalanceUpdate:
		if _, ok := payload.(*BalanceUpdateSnapshot); !ok {
			return fmt.Errorf("perp: kind %s requires *BalanceUpdateSnapshot", kind)
		}
	case UserTradeUpdate:
		if _, ok := payload.(*TradeSnapshot); !ok {
			return fmt.Errorf("perp: kind %s requires *TradeSnapshot", kind)
		}
	case UserPositionUpdate:
		if _, ok := payload.(*PositionSnapshot); !ok {
			return fmt.Errorf("perp: kind %s requires *PositionSnapshot", kind)
		}
	default:
		return fmt.Errorf("perp: unknown user kind %s", kind)
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

// Balance 在 Kind 为 UserBalanceUpdate 时返回余额变更载荷。
func (e *UserEvent) Balance() (*BalanceUpdateSnapshot, bool) {
	if e == nil || e.Kind != UserBalanceUpdate {
		return nil, false
	}
	b, ok := e.Payload.(*BalanceUpdateSnapshot)
	return b, ok
}

// Order 在 Kind 为 UserOrderUpdate 时返回订单载荷。
func (e *UserEvent) Order() (*OrderSnapshot, bool) {
	if e == nil || e.Kind != UserOrderUpdate {
		return nil, false
	}
	o, ok := e.Payload.(*OrderSnapshot)
	return o, ok
}

// Trade 在 Kind 为 UserTradeUpdate 时返回成交载荷。
func (e *UserEvent) Trade() (*TradeSnapshot, bool) {
	if e == nil || e.Kind != UserTradeUpdate {
		return nil, false
	}
	t, ok := e.Payload.(*TradeSnapshot)
	return t, ok
}

// Position 在 Kind 为 UserPositionUpdate 时返回持仓载荷。
func (e *UserEvent) Position() (*PositionSnapshot, bool) {
	if e == nil || e.Kind != UserPositionUpdate {
		return nil, false
	}
	p, ok := e.Payload.(*PositionSnapshot)
	return p, ok
}
