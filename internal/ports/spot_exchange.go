package ports

import (
	"context"

	"github.com/kainhuck/signalix/pkg/exchange/spot"
)

// SpotExchange 引擎所需的现货交易所能力；与 spot.Live 方法集对齐，
// 并包含建立链路的 Connect（*gateio.Client 实现；测试替身可 no-op）。
type SpotExchange interface {
	spot.Live
	Connect(ctx context.Context, parts spot.ConnectParts) error
}
