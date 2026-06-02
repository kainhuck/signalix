package market

import (
	"context"
	"testing"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

// mockMarket 是一个 inline 实现，用于在编译期验证 Market 接口签名自洽
// （5 个小接口可被单一实现满足），并确认接口仅依赖中性类型。
type mockMarket struct {
	kind     models.Market
	updates  chan MarketUpdate
	orderEvs chan *models.OrderEvent
}

func newMockMarket(kind models.Market) *mockMarket {
	return &mockMarket{
		kind:     kind,
		updates:  make(chan MarketUpdate),
		orderEvs: make(chan *models.OrderEvent),
	}
}

func (m *mockMarket) Kind() models.Market         { return m.kind }
func (m *mockMarket) Start(context.Context) error { return nil }
func (m *mockMarket) Stop() error                 { return nil }
func (m *mockMarket) Ping(context.Context) error  { return nil }

func (m *mockMarket) Subscribe(context.Context, SubscribeRequest) error { return nil }
func (m *mockMarket) Unsubscribe(string) error                          { return nil }
func (m *mockMarket) WarmupHistory(context.Context, SubscribeRequest, int) (*models.HistoryPayload, error) {
	return nil, nil
}
func (m *mockMarket) Updates() <-chan MarketUpdate { return m.updates }

func (m *mockMarket) Decide(context.Context, string, *models.Signal) (*models.Order, error) {
	return nil, nil
}

func (m *mockMarket) Place(context.Context, *models.Order) (string, error) { return "", nil }
func (m *mockMarket) Cancel(context.Context, *models.Order) error          { return nil }
func (m *mockMarket) Sync(context.Context, *models.Order) (*models.OrderEvent, error) {
	return nil, nil
}
func (m *mockMarket) OrderEvents() <-chan *models.OrderEvent { return m.orderEvs }

func (m *mockMarket) BuildRiskContext(context.Context, string, *models.Signal, *models.Order) (*ports.RiskContext, error) {
	return &ports.RiskContext{}, nil
}

func (m *mockMarket) Balance(context.Context, string) (*models.BalanceView, error) { return nil, nil }
func (m *mockMarket) Position(context.Context, string) (*models.PositionView, error) {
	return nil, nil
}
func (m *mockMarket) Ticker(string) (*models.Ticker, error)               { return nil, nil }
func (m *mockMarket) Klines(string, string, int) ([]*models.Kline, error) { return nil, nil }
func (m *mockMarket) ListPositions(context.Context) ([]*models.PositionView, error) {
	return nil, nil
}
func (m *mockMarket) ListTickers() (map[string]*models.Ticker, error) { return nil, nil }

// 编译期断言：mockMarket 同时满足聚合接口与全部窄接口。
var (
	_ Market         = (*mockMarket)(nil)
	_ MarketFeed     = (*mockMarket)(nil)
	_ MarketDecider  = (*mockMarket)(nil)
	_ MarketExecutor = (*mockMarket)(nil)
	_ MarketRisk     = (*mockMarket)(nil)
	_ MarketAccount  = (*mockMarket)(nil)
)

func TestMarketInterfaceComposition(t *testing.T) {
	var m Market = newMockMarket(models.MarketPerp)
	if m.Kind() != models.MarketPerp {
		t.Fatalf("Kind = %q, want perp", m.Kind())
	}
	// 窄接口可独立引用同一实现
	var feed MarketFeed = m
	if feed.Updates() == nil {
		t.Fatal("Updates channel must be non-nil")
	}
	var exec MarketExecutor = m
	if exec.OrderEvents() == nil {
		t.Fatal("OrderEvents channel must be non-nil")
	}
}
