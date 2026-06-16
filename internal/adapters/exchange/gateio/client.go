package gateio

import (
	"github.com/kainhuck/signalix/internal/ports"
	perpgate "github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
)

// NewClient 薄包装：返回实现 ports.PerpExchange 的 Gate 永续客户端。
func NewClient(apiKey, apiSecret string, opts ...perpgate.Option) ports.PerpExchange {
	return perpgate.NewClient(apiKey, apiSecret, opts...)
}
