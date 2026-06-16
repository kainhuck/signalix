package gateio

import (
	"github.com/kainhuck/signalix/internal/ports"
	perpgate "github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
	spotgate "github.com/kainhuck/signalix/pkg/exchange/spot/gateio"
)

// NewPerpClient 薄包装：返回实现 ports.PerpExchange 的 Gate 永续客户端。
func NewPerpClient(apiKey, apiSecret string, opts ...perpgate.Option) ports.PerpExchange {
	return perpgate.NewClient(apiKey, apiSecret, opts...)
}

// NewClient 保留旧入口，等价于 NewPerpClient。
func NewClient(apiKey, apiSecret string, opts ...perpgate.Option) ports.PerpExchange {
	return NewPerpClient(apiKey, apiSecret, opts...)
}

// NewSpotClient 薄包装：返回实现 ports.SpotExchange 的 Gate 现货客户端。
func NewSpotClient(apiKey, apiSecret string, opts ...spotgate.Option) ports.SpotExchange {
	return spotgate.NewClient(apiKey, apiSecret, opts...)
}
