package spot

import (
	"context"
	"fmt"
	"sync"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
)

// SpotMarket 聚合 spot 的 Feed / Decider / Executor / Risk / Account。
type SpotMarket struct {
	exchange ports.SpotExchange
	router   *MarketRouter
	proj     *AccountProjection

	decider  market.MarketDecider
	executor *SpotExecutor
	risk     market.MarketRisk

	marketBuf       int
	klineHistoryMax int
	routerWg        sync.WaitGroup
}

var _ market.Market = (*SpotMarket)(nil)

// SpotMarketConfig 构建 spot 市场所需依赖。
type SpotMarketConfig struct {
	Exchange            ports.SpotExchange
	DecisionSizeDivisor int
}

// NewSpotMarket 构造 spot 市场骨架。
func NewSpotMarket(ctx context.Context, cfg SpotMarketConfig, opts ...Option) (*SpotMarket, error) {
	if cfg.Exchange == nil {
		return nil, fmt.Errorf("spot exchange is required")
	}
	sm := &SpotMarket{
		exchange:        cfg.Exchange,
		marketBuf:       DefaultMarketBuffer,
		klineHistoryMax: DefaultKlineHistoryMax,
	}
	for _, o := range opts {
		o(sm)
	}

	pairs, err := cfg.Exchange.ListPairMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("spot pair meta load: %w", err)
	}
	pairMeta := pairMetaMap(pairs)
	sm.router = NewMarketRouter(cfg.Exchange, pairMeta,
		WithRouterBuffer(sm.marketBuf),
		WithRouterKlineHistoryMax(sm.klineHistoryMax),
	)
	sm.proj = NewAccountProjection(cfg.Exchange, pairMeta)
	sm.decider = NewSpotDecider(SpotDeciderConfig{
		Projection:         sm.proj,
		Router:             sm.router,
		PairMeta:           pairMeta,
		DefaultSizeDivisor: cfg.DecisionSizeDivisor,
	})
	executor, err := NewSpotExecutor(SpotExecutorConfig{Exchange: cfg.Exchange, Projection: sm.proj})
	if err != nil {
		return nil, err
	}
	sm.executor = executor
	sm.risk = NewSpotRisk(SpotRiskConfig{Projection: sm.proj, Router: sm.router})
	return sm, nil
}

func pairMetaMap(list []*spotex.PairMeta) map[spotex.Pair]*spotex.PairMeta {
	out := make(map[spotex.Pair]*spotex.PairMeta, len(list))
	for _, m := range list {
		if m == nil || m.Pair == "" {
			continue
		}
		cp := *m
		out[m.Pair.Canonical()] = &cp
	}
	return out
}

func (s *SpotMarket) Kind() models.Market { return models.MarketSpot }

func (s *SpotMarket) Ping(ctx context.Context) error {
	if s == nil || s.exchange == nil {
		return fmt.Errorf("spot market not configured")
	}
	return s.exchange.Ping(ctx)
}

func (s *SpotMarket) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("spot market not configured")
	}
	if s.proj != nil {
		if err := s.proj.Start(ctx); err != nil {
			return err
		}
	}
	if s.router != nil {
		s.routerWg.Add(1)
		go func() {
			defer s.routerWg.Done()
			s.router.Start()
		}()
	}
	if s.executor != nil {
		if err := s.executor.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *SpotMarket) Stop() error {
	if s == nil {
		return nil
	}
	if s.executor != nil {
		if err := s.executor.Stop(); err != nil {
			return err
		}
	}
	if s.proj != nil {
		s.proj.Stop()
	}
	if s.router != nil {
		if err := s.router.Stop(); err != nil {
			return err
		}
	}
	s.routerWg.Wait()
	return nil
}

func (s *SpotMarket) Subscribe(ctx context.Context, req market.SubscribeRequest) error {
	if s == nil || s.router == nil {
		return fmt.Errorf("spot router not configured")
	}
	return s.router.Subscribe(ctx, req)
}

func (s *SpotMarket) Unsubscribe(strategy string) error {
	if s == nil || s.router == nil {
		return fmt.Errorf("spot router not configured")
	}
	return s.router.Unsubscribe(strategy)
}

func (s *SpotMarket) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	if s == nil || s.router == nil {
		return nil, fmt.Errorf("spot router not configured")
	}
	return s.router.WarmupHistory(ctx, req, bars)
}

func (s *SpotMarket) Updates() <-chan market.MarketUpdate {
	if s == nil || s.router == nil {
		ch := make(chan market.MarketUpdate)
		close(ch)
		return ch
	}
	return s.router.Updates()
}

func (s *SpotMarket) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	return s.decider.Decide(ctx, strategy, sig)
}

func (s *SpotMarket) Place(ctx context.Context, o *models.Order) (string, error) {
	return s.executor.Place(ctx, o)
}

func (s *SpotMarket) Cancel(ctx context.Context, o *models.Order) error {
	return s.executor.Cancel(ctx, o)
}

func (s *SpotMarket) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	return s.executor.Sync(ctx, o)
}

func (s *SpotMarket) OrderEvents() <-chan *models.OrderEvent {
	return s.executor.OrderEvents()
}

func (s *SpotMarket) BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error) {
	if s.risk == nil {
		return nil, fmt.Errorf("spot risk not configured")
	}
	return s.risk.BuildRiskContext(ctx, strategy, sig, o)
}

func (s *SpotMarket) BindRisk(cfg SpotRiskConfig) {
	if s == nil {
		return
	}
	cfg.Projection = s.proj
	cfg.Router = s.router
	s.risk = NewSpotRisk(cfg)
}

func (s *SpotMarket) Executor() market.MarketExecutor { return s.executor }
func (s *SpotMarket) Risk() market.MarketRisk         { return s.risk }
func (s *SpotMarket) Projection() *AccountProjection  { return s.proj }
func (s *SpotMarket) Exchange() ports.SpotExchange    { return s.exchange }
func (s *SpotMarket) Router() *MarketRouter           { return s.router }
