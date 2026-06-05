package spot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	spgate "github.com/kainhuck/signalix/pkg/exchange/spot/gateio"
)

type SpotMarket struct {
	client  *spgate.Client
	router  *SpotRouter
	decider market.MarketDecider

	routerWg sync.WaitGroup
}

var _ market.Market = (*SpotMarket)(nil)

type SpotMarketConfig struct {
	Client    *spgate.Client
	MarketBuf int
}

func NewSpotMarket(ctx context.Context, cfg SpotMarketConfig) (*SpotMarket, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("spot client is required")
	}
	marketBuf := cfg.MarketBuf
	if marketBuf <= 0 {
		marketBuf = 1000
	}

	router := NewSpotRouter(cfg.Client, WithBufferSize(marketBuf))

	return &SpotMarket{
		client:  cfg.Client,
		router:  router,
		decider: newSpotDecider(),
	}, nil
}

func (s *SpotMarket) Kind() models.Market { return models.MarketSpot }

func (s *SpotMarket) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("spot market not configured")
	}
	return s.client.Ping(ctx)
}

func (s *SpotMarket) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("spot market not configured")
	}
	s.routerWg.Add(1)
	go func() {
		defer s.routerWg.Done()
		s.router.Start()
	}()
	return nil
}

func (s *SpotMarket) Stop() error {
	if s == nil {
		return nil
	}
	if s.router != nil {
		if err := s.router.Stop(); err != nil {
			return err
		}
	}
	s.routerWg.Wait()
	return nil
}

func (s *SpotMarket) Router() *SpotRouter { return s.router }

func (s *SpotMarket) Subscribe(ctx context.Context, req market.SubscribeRequest) error {
	return s.router.Subscribe(ctx, req)
}

func (s *SpotMarket) Unsubscribe(strategy string) error {
	return s.router.Unsubscribe(strategy)
}

func (s *SpotMarket) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	return s.router.WarmupHistory(ctx, req, bars)
}

func (s *SpotMarket) Updates() <-chan market.MarketUpdate {
	return s.router.Updates()
}

func (s *SpotMarket) Decide(ctx context.Context, strategy string, sig *models.Signal) (*models.Order, error) {
	return s.decider.Decide(ctx, strategy, sig)
}

func (s *SpotMarket) Place(ctx context.Context, o *models.Order) (string, error) {
	return "", fmt.Errorf("spot executor not implemented")
}

func (s *SpotMarket) Cancel(ctx context.Context, o *models.Order) error {
	return fmt.Errorf("spot executor not implemented")
}

func (s *SpotMarket) Sync(ctx context.Context, o *models.Order) (*models.OrderEvent, error) {
	return nil, fmt.Errorf("spot executor not implemented")
}

func (s *SpotMarket) OrderEvents() <-chan *models.OrderEvent {
	return nil
}

func (s *SpotMarket) BuildRiskContext(ctx context.Context, strategy string, sig *models.Signal, o *models.Order) (*ports.RiskContext, error) {
	return nil, fmt.Errorf("spot risk not implemented")
}

func (s *SpotMarket) Balance(ctx context.Context, currency string) (*models.BalanceView, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("spot market not configured")
	}
	bv, err := s.client.Balance(ctx, currency)
	if err != nil {
		return nil, err
	}
	if bv == nil {
		return nil, nil
	}
	return &models.BalanceView{
		Currency:  bv.Currency,
		Total:     bv.Total,
		Available: bv.Available,
		Frozen:    bv.Frozen,
		UpdatedAt: bv.UpdatedAt,
	}, nil
}

func (s *SpotMarket) Position(ctx context.Context, symbol string) (*models.PositionView, error) {
	return nil, nil
}

func (s *SpotMarket) Ticker(symbol string) (*models.Ticker, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	pair := spot.CanonicalPair(strings.TrimSpace(symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	snap, ok := s.router.GetCachedTicker(pair)
	if !ok {
		return nil, fmt.Errorf("%w: %s", market.ErrTickerNotInCache, pair)
	}
	return spotTickerToModel(snap), nil
}

func (s *SpotMarket) Klines(symbol, interval string, limit int) ([]*models.Kline, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	pair := spot.CanonicalPair(strings.TrimSpace(symbol))
	if pair == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	interval = strings.TrimSpace(interval)
	if interval == "" {
		return nil, fmt.Errorf("interval is required")
	}
	limit = normalizeKlineLimit(limit)
	klines := s.router.ListClosedKlines(pair, interval, limit)
	if klines == nil {
		return []*models.Kline{}, nil
	}
	return klines, nil
}

func (s *SpotMarket) ListPositions(ctx context.Context) ([]*models.PositionView, error) {
	return nil, nil
}

func (s *SpotMarket) ListTickers() (map[string]*models.Ticker, error) {
	if s == nil || s.router == nil {
		return nil, market.ErrMarketRouterNotConfigured
	}
	raw := s.router.GetAllCachedTickers()
	out := make(map[string]*models.Ticker, len(raw))
	for pair, snap := range raw {
		if snap == nil {
			continue
		}
		out[string(pair)] = spotTickerToModel(snap)
	}
	return out, nil
}

func normalizeKlineLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}
