package gateio

import (
	"github.com/kainhuck/signalix/internal/ports"
	perpgate "github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
)

// NewPerpClient 薄包装：返回实现 ports.PerpExchange 的 Gate 永续客户端。
func NewPerpClient(apiKey, apiSecret string, opts ...perpgate.Option) ports.PerpExchange {
	return perpgate.NewClient(apiKey, apiSecret, opts...)
}
