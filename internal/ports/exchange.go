package ports

import (
	"context"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

// PerpExchange engine 所需的永续交易所能力。
// 由 *gateio.Client 等适配器实现，便于构造注入与测试替身。
type PerpExchange interface {
	perp.Live
	perp.ClientOrderIDCodec
	Connect(ctx context.Context, parts perp.ConnectParts) error
}

// SpotExchange engine 所需的现货交易所能力。
type SpotExchange interface {
	spot.Live
	Connect(ctx context.Context, parts spot.ConnectParts) error
}
