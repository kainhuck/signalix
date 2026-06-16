package ports

import (
	"context"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// PerpExchange 引擎所需的永续交易所能力；一期与 perp.Live 方法集对齐，
// 并包含建立链路的 Connect（*gateio.Client 实现；测试替身可 no-op）。
// 由 *gateio.Client 等适配器实现，便于构造注入与测试替身。
type PerpExchange interface {
	perp.Live
	perp.ClientOrderIDCodec
	Connect(ctx context.Context, parts perp.ConnectParts) error
}
