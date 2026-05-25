package ports

import (
	"context"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

// OrderStore 本地订单与策略状态持久化（SQLite 等实现）；nil 表示禁用。
type OrderStore interface {
	SaveOrder(ctx context.Context, order *models.Order) error
	ListNonTerminalOrders(ctx context.Context) ([]*models.Order, error)
	SaveStrategyState(ctx context.Context, strategyName, stateKey string, valueJSON []byte, updatedAt time.Time) error
	LoadAllStrategyStates(ctx context.Context) (map[string]map[string][]byte, error)
	Close() error
}
