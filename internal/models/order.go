package models

import (
	"time"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// Order 订单
type Order struct {
	ID           string        `json:"id"`
	ExchangeID   string        `json:"exchange_id,omitempty"`
	Symbol       perp.Contract `json:"symbol"`
	Side         OrderSide     `json:"side"`
	OrderType    OrderType     `json:"order_type"`
	Price        *string       `json:"price,omitempty"`
	Size         string        `json:"size"`
	FilledSize   string        `json:"filled_size"`
	Status       OrderStatus   `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	StopPrice    *string       `json:"stop_price,omitempty"`
	StrategyName string        `json:"strategy_name"`
}

type OrderSide string

const (
	OrderSideBuy  OrderSide = "Buy"
	OrderSideSell OrderSide = "Sell"
)

type OrderType string

const (
	OrderTypeMarket OrderType = "Market"
	OrderTypeLimit  OrderType = "Limit"
)

type OrderStatus string

const (
	OrderStatusPending       OrderStatus = "Pending"
	OrderStatusSubmitted     OrderStatus = "Submitted"
	OrderStatusPartialFilled OrderStatus = "PartialFilled"
	OrderStatusFilled        OrderStatus = "Filled"
	OrderStatusCancelled     OrderStatus = "Cancelled"
	OrderStatusRejected      OrderStatus = "Rejected"
)
