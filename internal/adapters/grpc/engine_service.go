package grpc

import (
	"context"
	"errors"
	"runtime"
	"sort"
	"strings"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EngineService 实现 signalix.engine.v1.Engine。
type EngineService struct {
	enginev1.UnimplementedEngineServer
	eng *engine.Engine
}

// NewEngineService 构造 gRPC 服务实现。
func NewEngineService(eng *engine.Engine) *EngineService {
	return &EngineService{eng: eng}
}

func (s *EngineService) Ping(ctx context.Context, _ *enginev1.PingRequest) (*enginev1.PingReply, error) {
	_ = ctx
	return &enginev1.PingReply{Pong: "pong", ServerTimeUnixMs: time.Now().UnixMilli()}, nil
}

func (s *EngineService) GetHealth(ctx context.Context, req *enginev1.GetHealthRequest) (*enginev1.GetHealthReply, error) {
	if s.eng == nil {
		return nil, status.Error(codes.Internal, "nil engine")
	}
	skip := false
	if req != nil {
		skip = req.GetSkipExchangePing()
	}
	report := s.eng.HealthReport(ctx, skip)
	return healthReportToProto(report), nil
}

func (s *EngineService) GetEngineInfo(ctx context.Context, _ *enginev1.GetEngineInfoRequest) (*enginev1.GetEngineInfoReply, error) {
	_ = ctx
	return &enginev1.GetEngineInfoReply{
		Version:       s.eng.BuildInfoVersion(),
		StrategiesDir: s.eng.GetStrategiesDir(),
		GoVersion:     runtime.Version(),
	}, nil
}

func (s *EngineService) ListStrategies(ctx context.Context, _ *enginev1.ListStrategiesRequest) (*enginev1.ListStrategiesReply, error) {
	_ = ctx
	list := s.eng.ListStrategyRuntimeSnapshots()
	out := make([]*enginev1.StrategySummary, 0, len(list))
	for _, snap := range list {
		out = append(out, strategySummaryToProto(snap))
	}
	return &enginev1.ListStrategiesReply{Strategies: out}, nil
}

func (s *EngineService) GetStrategyStatus(ctx context.Context, req *enginev1.GetStrategyStatusRequest) (*enginev1.GetStrategyStatusReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "empty name")
	}
	snap, err := s.eng.StrategyRuntimeSnapshot(name)
	if err != nil {
		if errors.Is(err, engine.ErrStrategyNotFound) {
			return nil, status.Errorf(codes.NotFound, "strategy %q not in catalog", name)
		}
		return nil, status.Errorf(codes.Internal, "strategy status: %v", err)
	}
	return &enginev1.GetStrategyStatusReply{Status: strategySummaryToProto(snap)}, nil
}

func (s *EngineService) StartStrategy(ctx context.Context, req *enginev1.StartStrategyRequest) (*enginev1.StartStrategyReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "empty name")
	}
	st, ok := s.eng.GetStrategy(name)
	if !ok || st == nil {
		return nil, status.Errorf(codes.NotFound, "strategy not in catalog: %s", name)
	}
	if !st.Enabled {
		return nil, status.Errorf(codes.FailedPrecondition, "strategy %q is disabled in config", name)
	}
	if err := s.eng.StartStrategyByName(name); err != nil {
		return nil, status.Errorf(codes.Internal, "start strategy: %v", err)
	}
	logger.InfoContext(ctx, "grpc StartStrategy", logger.String("strategy", name))
	return &enginev1.StartStrategyReply{}, nil
}

func (s *EngineService) StopStrategy(ctx context.Context, req *enginev1.StopStrategyRequest) (*enginev1.StopStrategyReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "empty name")
	}
	if err := s.eng.StopStrategy(name); err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	logger.InfoContext(ctx, "grpc StopStrategy", logger.String("strategy", name))
	return &enginev1.StopStrategyReply{}, nil
}

func (s *EngineService) ReloadStrategies(ctx context.Context, _ *enginev1.ReloadStrategiesRequest) (*enginev1.ReloadStrategiesReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	n, err := s.eng.ReloadStrategiesCatalog()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reload: %v", err)
	}
	logger.InfoContext(ctx, "grpc ReloadStrategies", logger.Int("catalog_count", n))
	return &enginev1.ReloadStrategiesReply{CatalogCount: int32(n)}, nil
}

func (s *EngineService) ListTemplates(ctx context.Context, _ *enginev1.ListTemplatesRequest) (*enginev1.ListTemplatesReply, error) {
	_ = ctx
	list, err := s.eng.ListStrategyTemplates()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list templates: %v", err)
	}
	out := make([]*enginev1.StrategyTemplate, 0, len(list))
	for _, m := range list {
		out = append(out, templateMetaToProto(m))
	}
	return &enginev1.ListTemplatesReply{Templates: out}, nil
}

func (s *EngineService) CreateStrategy(ctx context.Context, req *enginev1.CreateStrategyRequest) (*enginev1.CreateStrategyReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "empty name")
	}
	templateID := strings.TrimSpace(req.GetTemplateId())
	if templateID == "" {
		return nil, status.Error(codes.InvalidArgument, "empty template_id")
	}
	opts := createStrategyOptionsFromProto(req)
	opts.Name = name
	opts.TemplateID = templateID
	path, n, err := s.eng.CreateStrategyScaffold(opts)
	if err != nil {
		return nil, mapScaffoldErr(err)
	}
	logger.InfoContext(ctx, "grpc CreateStrategy",
		logger.String("name", name),
		logger.String("template", templateID),
		logger.String("path", path))
	return &enginev1.CreateStrategyReply{
		Name:         name,
		Path:         path,
		CatalogCount: int32(n),
	}, nil
}

func (s *EngineService) GetBalance(ctx context.Context, _ *enginev1.GetBalanceRequest) (*enginev1.GetBalanceReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	bal, err := s.eng.BalanceSnapshot()
	if err != nil {
		return nil, mapProjectionErr(err)
	}
	return &enginev1.GetBalanceReply{Balance: balanceToProto(bal)}, nil
}

func (s *EngineService) GetPosition(ctx context.Context, req *enginev1.GetPositionRequest) (*enginev1.GetPositionReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	symbol := strings.TrimSpace(req.GetSymbol())
	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "empty symbol")
	}
	pos, err := s.eng.PositionSnapshot(perp.Contract(symbol))
	if err != nil {
		return nil, mapProjectionErr(err)
	}
	if pos == nil {
		return &enginev1.GetPositionReply{}, nil
	}
	return &enginev1.GetPositionReply{Position: positionToProto(pos)}, nil
}

func (s *EngineService) ListPositions(ctx context.Context, _ *enginev1.ListPositionsRequest) (*enginev1.ListPositionsReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	list, err := s.eng.AllPositionsSnapshot()
	if err != nil {
		return nil, mapProjectionErr(err)
	}
	out := make([]*enginev1.Position, 0, len(list))
	for _, p := range list {
		out = append(out, positionToProto(p))
	}
	return &enginev1.ListPositionsReply{Positions: out}, nil
}

func (s *EngineService) GetTicker(ctx context.Context, req *enginev1.GetTickerRequest) (*enginev1.GetTickerReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	symbol := strings.TrimSpace(req.GetSymbol())
	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "empty symbol")
	}
	snap, err := s.eng.TickerSnapshot(perp.Contract(symbol))
	if err != nil {
		return nil, mapMarketErr(err)
	}
	return &enginev1.GetTickerReply{Ticker: tickerToProto(snap)}, nil
}

func (s *EngineService) GetKlines(ctx context.Context, req *enginev1.GetKlinesRequest) (*enginev1.GetKlinesReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	symbol := strings.TrimSpace(req.GetSymbol())
	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "empty symbol")
	}
	interval := strings.TrimSpace(req.GetInterval())
	if interval == "" {
		return nil, status.Error(codes.InvalidArgument, "empty interval")
	}
	klines, err := s.eng.ClosedKlines(perp.Contract(symbol), interval, int(req.GetLimit()))
	if err != nil {
		return nil, mapMarketErr(err)
	}
	out := make([]*enginev1.Kline, 0, len(klines))
	for _, k := range klines {
		out = append(out, klineToProto(k))
	}
	return &enginev1.GetKlinesReply{Klines: out}, nil
}

func (s *EngineService) ListTickers(ctx context.Context, _ *enginev1.ListTickersRequest) (*enginev1.ListTickersReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	all, err := s.eng.ListCachedTickers()
	if err != nil {
		return nil, mapMarketErr(err)
	}
	symbols := make([]string, 0, len(all))
	for sym := range all {
		symbols = append(symbols, string(sym))
	}
	sort.Strings(symbols)
	out := make([]*enginev1.Ticker, 0, len(symbols))
	for _, sym := range symbols {
		out = append(out, tickerToProto(all[perp.Contract(sym)]))
	}
	return &enginev1.ListTickersReply{Tickers: out}, nil
}

func (s *EngineService) GetOrder(ctx context.Context, req *enginev1.GetOrderRequest) (*enginev1.GetOrderReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetOrderId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "empty order_id")
	}
	o, ok := s.eng.GetOrderSnapshot(id)
	if !ok || o == nil {
		return nil, status.Errorf(codes.NotFound, "order %q not found", id)
	}
	return &enginev1.GetOrderReply{Order: orderToProto(o)}, nil
}

func (s *EngineService) ListOpenOrders(ctx context.Context, req *enginev1.ListOpenOrdersRequest) (*enginev1.ListOpenOrdersReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	limit := int(req.GetLimit())
	orders := s.eng.ListOpenOrdersSnapshot(limit)
	out := make([]*enginev1.Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToProto(o))
	}
	return &enginev1.ListOpenOrdersReply{Orders: out}, nil
}

func (s *EngineService) CancelOrder(ctx context.Context, req *enginev1.CancelOrderRequest) (*enginev1.CancelOrderReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetOrderId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "empty order_id")
	}
	if err := s.eng.CancelOrderViaOMS(ctx, id); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "cancel: %v", err)
	}
	logger.InfoContext(ctx, "grpc CancelOrder", logger.String("order_id", id))
	return &enginev1.CancelOrderReply{}, nil
}

func (s *EngineService) ActivateKillSwitch(ctx context.Context, req *enginev1.ActivateKillSwitchRequest) (*enginev1.ActivateKillSwitchReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	result, err := s.eng.ActivateKillSwitch(ctx, req.GetReason(), req.GetCancelOpenOrders())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "activate kill switch: %v", err)
	}
	logger.InfoContext(ctx, "grpc ActivateKillSwitch",
		logger.String("reason", result.Status.Reason),
		logger.Bool("active", result.Status.Active),
		logger.Int("cancel_attempted", result.CancelAttempted),
		logger.Int("cancel_failed", result.CancelFailed))
	return &enginev1.ActivateKillSwitchReply{
		Status:          killSwitchStatusToProto(result.Status),
		CancelAttempted: int32(result.CancelAttempted),
		CancelFailed:    int32(result.CancelFailed),
	}, nil
}

func (s *EngineService) DeactivateKillSwitch(ctx context.Context, _ *enginev1.DeactivateKillSwitchRequest) (*enginev1.DeactivateKillSwitchReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	st, err := s.eng.DeactivateKillSwitch(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deactivate kill switch: %v", err)
	}
	logger.InfoContext(ctx, "grpc DeactivateKillSwitch", logger.Bool("active", st.Active))
	return &enginev1.DeactivateKillSwitchReply{Status: killSwitchStatusToProto(st)}, nil
}

func (s *EngineService) GetKillSwitchStatus(ctx context.Context, _ *enginev1.GetKillSwitchStatusRequest) (*enginev1.GetKillSwitchStatusReply, error) {
	if err := requireRunning(s.eng); err != nil {
		return nil, err
	}
	st := s.eng.KillSwitchStatus()
	return &enginev1.GetKillSwitchStatusReply{Status: killSwitchStatusToProto(st)}, nil
}

func killSwitchStatusToProto(st engine.KillSwitchStatus) *enginev1.KillSwitchStatus {
	po := &enginev1.KillSwitchStatus{
		Active: st.Active,
		Reason: st.Reason,
	}
	if st.Active {
		po.ActivatedAtUnixMs = st.ActivatedAt.UnixMilli()
	}
	return po
}

func (s *EngineService) SubscribeOrderEvents(req *enginev1.SubscribeOrderEventsRequest, stream grpc.ServerStreamingServer[enginev1.Order]) error {
	_ = req
	if err := requireRunning(s.eng); err != nil {
		return err
	}
	ctx := stream.Context()
	ch := s.eng.RegisterOrderSubscriber(ctx, 64)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case o, ok := <-ch:
			if !ok {
				return nil
			}
			if o == nil {
				continue
			}
			if err := stream.Send(orderToProto(o)); err != nil {
				return err
			}
		}
	}
}

func requireRunning(eng *engine.Engine) error {
	if eng == nil {
		return status.Error(codes.Internal, "nil engine")
	}
	if !eng.Running() {
		return status.Error(codes.FailedPrecondition, "engine not running")
	}
	return nil
}

func orderToProto(o *models.Order) *enginev1.Order {
	if o == nil {
		return nil
	}
	po := &enginev1.Order{
		Id:              o.ID,
		ExchangeId:      o.ExchangeID,
		Symbol:          string(o.Symbol),
		Side:            string(o.Side),
		OrderType:       string(o.OrderType),
		Size:            o.Size,
		FilledSize:      o.FilledSize,
		Status:          string(o.Status),
		StrategyName:    o.StrategyName,
		CreatedAtUnixMs: o.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs: o.UpdatedAt.UnixMilli(),
	}
	if o.Price != nil {
		po.Price = *o.Price
	}
	if o.StopPrice != nil {
		po.StopPrice = *o.StopPrice
	}
	return po
}
