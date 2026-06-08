package gateio

import (
	"github.com/kainhuck/signalix/internal/ports"
	spotgate "github.com/kainhuck/signalix/pkg/exchange/spot/gateio"
)

// NewSpotClient 薄包装：返回实现 ports.SpotExchange 的 Gate 现货客户端。
func NewSpotClient(apiKey, apiSecret string, opts ...spotgate.Option) ports.SpotExchange {
	return spotgate.NewClient(apiKey, apiSecret, opts...)
}
