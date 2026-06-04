package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	exad "github.com/kainhuck/signalix/internal/adapters/exchange/gateio"
	signalixgrpc "github.com/kainhuck/signalix/internal/adapters/grpc"
	"github.com/kainhuck/signalix/internal/adapters/store/sqlite"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/market"
	mktperp "github.com/kainhuck/signalix/internal/app/market/perp"
	apprisk "github.com/kainhuck/signalix/internal/app/risk"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	perpgate "github.com/kainhuck/signalix/pkg/exchange/perp/gateio"
	"github.com/kainhuck/signalix/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logOpts, err := cfg.Log.LoggerOptions()
	if err != nil {
		_, _ = os.Stderr.WriteString("log config: " + err.Error() + "\n")
		os.Exit(1)
	}
	if err := logger.Init(logOpts...); err != nil {
		_, _ = os.Stderr.WriteString("logger init: " + err.Error() + "\n")
		os.Exit(1)
	}

	ex := cfg.Exchange
	gateOpts := []perpgate.Option{
		perpgate.WithPaper(ex.Paper),
		perpgate.WithSettle(ex.Settle),
		perpgate.WithUserID(ex.UserID),
		perpgate.WithLogger(logger.With("exchange", "gateio")),
		perpgate.WithChannelBuffers(ex.PublicWSBuffer, ex.PrivateWSBuffer),
		perpgate.WithRateLimit(ex.RateLimit),
	}
	if ex.RESTBasePath != "" {
		gateOpts = append(gateOpts, perpgate.WithRESTBasePath(ex.RESTBasePath))
	}
	if ex.Proxy != "" {
		gateOpts = append(gateOpts, perpgate.WithProxy(ex.Proxy))
	}

	c := exad.NewClient(ex.APIKey, ex.APISecret, gateOpts...)
	parts := perp.ConnectParts{
		REST:      ex.Connect.REST,
		PublicWS:  ex.Connect.PublicWS,
		PrivateWS: ex.Connect.PrivateWS,
	}
	if err := c.Connect(context.Background(), parts); err != nil {
		logger.Error("connect failed", "error", err)
		return
	}

	st, err := sqlite.Open(cfg.DatabasePath, cfg.Database.MaxOpenConns)
	if err != nil {
		logger.Error("sqlite open failed", "path", cfg.DatabasePath, "error", err)
		return
	}

	enabled, err := cfg.EnabledMarkets()
	if err != nil {
		logger.Error("markets config", "error", err)
		return
	}

	build := engine.BuildParamsFromConfig(cfg)
	markets := make(map[models.Market]market.Market)
	var perpMarket *mktperp.PerpMarket
	equityTracker := apprisk.NewEquityTracker()
	for _, m := range enabled {
		switch m {
		case models.MarketPerp:
			pm, err := mktperp.NewPerpMarket(context.Background(), mktperp.PerpMarketConfig{
				Exchange:          c,
				MarketBuf:         cfg.Channels.Market,
				DecisionDivisor:   cfg.Decision.DefaultSizeDivisor,
				ProjectionRefresh: cfg.ProjectionRefreshInterval(),
			})
			if err != nil {
				logger.Error("perp market init failed", "error", err)
				return
			}
			pm.AttachEquityHook(equityTracker.OnEquityUpdate)
			perpMarket = pm
			build.AccountProjection = pm.Projection()
			build.MetaLookup = pm.Registry()
			markets[m] = pm
		default:
			logger.Error("unsupported market", "market", m)
			return
		}
	}
	if len(markets) == 0 {
		logger.Error("no markets enabled")
		return
	}

	eng := engine.NewEngine(cfg.StrategiesDir, markets, build,
		engine.WithPersistence(st),
		engine.WithRiskRules(cfg.RiskRules()),
	)
	if perpMarket != nil {
		perpMarket.BindRisk(mktperp.PerpRiskConfig{
			Proj:          perpMarket.Projection(),
			Decision:      perpMarket.DecisionEngine(),
			Execution:     eng.ExecutionEngine(),
			Equity:        equityTracker,
			NeedsNotional: eng,
		})
		if st != nil && perpMarket.Projection() != nil {
			perpMarket.Projection().SetRefreshHook(
				mktperp.WrapProjectionRefreshHook(eng.PersistAccountSnapshot),
			)
		}
	}
	if err := eng.Start(); err != nil {
		logger.Error("failed to start engine", "error", err)
		return
	}
	defer eng.Stop()

	sigCtx, stopSig := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSig()

	if cfg.GRPC.Enabled {
		logger.Info("grpc listening (blocking)", "addr", cfg.GRPC.Addr)
		if err := signalixgrpc.ServeBlocking(sigCtx, eng, signalixgrpc.Options{
			Addr:            cfg.GRPC.Addr,
			Token:           cfg.GRPC.Token,
			InsecureBindAll: cfg.GRPC.InsecureBindAll,
		}); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("grpc serve", "error", err)
		}
		return
	}

	logger.Info("signalixd running without gRPC; send SIGINT/SIGTERM to exit",
		"work_dir", cfg.WorkDir)
	<-sigCtx.Done()
}
