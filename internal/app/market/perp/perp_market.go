package perp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/decision"
	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/logger"
)

// PerpMarketConfig 构建 perp 市场所需的依赖。
type PerpMarketConfig struct {
	Exchange          ports.PerpExchange
	MarketBuf         int
	DecisionDivisor   int
	ProjectionRefresh time.Duration
	EquityHook        projection.EquityHook
}

// PerpMarket 聚合 perp 的 Feed / Decider / Executor / Risk / Account。
type PerpMarket struct {
	exchange ports.PerpExchange
	router   *MarketRouter
	decider  market.MarketDecider
	executor *PerpExecutor
	risk     market.MarketRisk
	proj     *projection.AccountProjection
	reg      *instrument.Registry
	de       *decision.DecisionEngine

	routerWg sync.WaitGroup
}

var _ market.Market = (*PerpMarket)(nil)

// NewPerpMarket 构造 perp 市场（不含 market.MarketRisk；须再调用 BindRisk）。
func NewPerpMarket(ctx context.Context, cfg PerpMarketConfig) (*PerpMarket, error) {
	if cfg.Exchange == nil {
		return nil, fmt.Errorf("exchange is required")
	}
	marketBuf := cfg.MarketBuf
	if marketBuf <= 0 {
		marketBuf = 1000
	}
	divisor := cfg.DecisionDivisor
	if divisor <= 0 {
		divisor = 10
	}
	refresh := cfg.ProjectionRefresh
	if refresh <= 0 {
		refresh = projection.DefaultProjectionRefreshInterval
	}

	projOpts := []projection.Option{projection.WithRefreshInterval(refresh)}
	if cfg.EquityHook != nil {
		projOpts = append(projOpts, projection.WithEquityHook(cfg.EquityHook))
	}
	proj := projection.NewAccountProjection(cfg.Exchange, projOpts...)

	router := NewMarketRouter(cfg.Exchange, WithBufferSize(marketBuf))

	reg := instrument.NewRegistry()
	if err := reg.LoadFrom(ctx, cfg.Exchange); err != nil {
		logger.WarnContext(ctx, "contract meta registry load failed", logger.Any("error", err))
	}

	de := decision.NewDecisionEngine(proj,
		decision.WithDefaultSizeDivisor(divisor),
		decision.WithContractMetaLookup(reg),
		decision.WithTickerLookup(router),
	)

	executor, err := NewPerpExecutor(PerpExecutorConfig{Exchange: cfg.Exchange, Proj: proj})
	if err != nil {
		return nil, err
	}

	return &PerpMarket{
		exchange: cfg.Exchange,
		router:   router,
		decider:  NewPerpDecider(de),
		executor: executor,
		proj:     proj,
		reg:      reg,
		de:       de,
	}, nil
}

// BindRisk 在 OMS 就绪后绑定 market.MarketRisk（依赖 ExecutionEngine）。
func (p *PerpMarket) BindRisk(cfg PerpRiskConfig) {
	if p == nil {
		return
	}
	cfg.Proj = p.proj
	cfg.Decision = p.de
	p.risk = NewPerpRisk(cfg)
}

// Kind 满足 Market。
func (p *PerpMarket) Kind() models.Market { return models.MarketPerp }

// Ping 满足 market.Market。
func (p *PerpMarket) Ping(ctx context.Context) error {
	if p == nil || p.exchange == nil {
		return fmt.Errorf("perp market not configured")
	}
	return p.exchange.Ping(ctx)
}

// DecisionEngine 返回 perp 决策引擎（供 main BindRisk 使用）。
func (p *PerpMarket) DecisionEngine() *decision.DecisionEngine {
	if p == nil {
		return nil
	}
	return p.de
}

// Start 启动行情泵与用户流泵。
func (p *PerpMarket) Start(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("perp market not configured")
	}
	p.routerWg.Add(1)
	go func() {
		defer p.routerWg.Done()
		p.router.Start()
	}()
	return p.executor.Start(ctx)
}

// Stop 停止 router 与 executor。
func (p *PerpMarket) Stop() error {
	if p == nil {
		return nil
	}
	if p.executor != nil {
		if err := p.executor.Stop(); err != nil {
			return err
		}
	}
	if p.router != nil {
		if err := p.router.Stop(); err != nil {
			return err
		}
	}
	p.routerWg.Wait()
	return nil
}

// AttachEquityHook 在 projection 启动前注册权益钩子。
func (p *PerpMarket) AttachEquityHook(h projection.EquityHook) {
	if p == nil || p.proj == nil || h == nil {
		return
	}
	p.proj.SetEquityHook(h)
}

func (p *PerpMarket) Executor() market.MarketExecutor { return p.executor }
func (p *PerpMarket) Risk() market.MarketRisk         { return p.risk }
func (p *PerpMarket) Projection() *projection.AccountProjection {
	return p.proj
}
func (p *PerpMarket) Registry() *instrument.Registry { return p.reg }
func (p *PerpMarket) Exchange() ports.PerpExchange       { return p.exchange }
func (p *PerpMarket) Router() *MarketRouter          { return p.router }

// --- market.MarketFeed ---

func (p *PerpMarket) Subscribe(ctx context.Context, req market.SubscribeRequest) error {
	return p.router.Subscribe(ctx, req)
}

func (p *PerpMarket) Unsubscribe(strategy string) error {
	return p.router.Unsubscribe(strategy)
}

func (p *PerpMarket) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	return p.router.WarmupHistory(ctx, req, bars)
}

func (p *PerpMarket) Updates() <-chan market.MarketUpdate {
	return p.router.Updates()
}

// --- market.MarketDecider ---

func (p *PerpMarket) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	return p.decider.Decide(ctx, strategy, sig)
}

// --- market.MarketExecutor ---

func (p *PerpMarket) Place(ctx context.Context, o *models.Order) (string, error) {
	return p.executor.Place(ctx, o)
}

func (p *PerpMarket) Cancel(ctx context.Context, o *models.Order) error {
	return p.executor.Cancel(ctx, o)
}

func (p *PerpMarket) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	return p.executor.Sync(ctx, o)
}

func (p *PerpMarket) OrderEvents() <-chan *models.OrderEvent {
	return p.executor.OrderEvents()
}

// --- market.MarketRisk ---

func (p *PerpMarket) BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error) {
	if p.risk == nil {
		return nil, fmt.Errorf("perp risk not configured")
	}
	return p.risk.BuildRiskContext(ctx, strategy, sig, o)
}

// PerpFromMarkets 从注册表取出 perp 市场实现。
func PerpFromMarkets(markets map[models.Market]market.Market) (*PerpMarket, error) {
	m, ok := markets[models.MarketPerp]
	if !ok || m == nil {
		return nil, fmt.Errorf("market %q not registered", models.MarketPerp)
	}
	pm, ok := m.(*PerpMarket)
	if !ok {
		return nil, fmt.Errorf("market %q is not *PerpMarket", models.MarketPerp)
	}
	return pm, nil
}
