package engine

import (
	"time"

	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/app/projection"
	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/domain/risk"
	"github.com/kainhuck/signalix/internal/ports"
)

// EngineOption 可选构造 Engine。
type EngineOption func(*Engine)

// BuildParams 创建 Engine 时子模块的运行参数（来自 config.toml）。
type BuildParams struct {
	Channels          config.ChannelsConfig
	ProjectionRefresh time.Duration
	DecisionDivisor   int
	OMSMaxRetries     int
	DefaultInterval   string
	Restart           config.RestartSettings
	KillSwitch        config.KillSwitchSettings
	Persistence       config.PersistenceSettings
	AccountProjection *projection.AccountProjection
	MetaLookup        instrument.ContractMetaLookup
}

// BuildParamsFromConfig 从全局配置提取构建参数。
func BuildParamsFromConfig(c *config.Config) BuildParams {
	if c == nil {
		return BuildParams{
			ProjectionRefresh: 10 * time.Second,
			DecisionDivisor:   10,
			OMSMaxRetries:     3,
			Restart:           config.DefaultRestartSettings(),
			KillSwitch:        config.DefaultKillSwitchSettings(),
			Persistence:       config.DefaultPersistenceSettings(),
		}
	}
	return BuildParams{
		Channels:          c.Channels,
		ProjectionRefresh: c.ProjectionRefreshInterval(),
		DecisionDivisor:   c.Decision.DefaultSizeDivisor,
		OMSMaxRetries:     c.Exchange.MaxRetries,
		DefaultInterval:   c.Strategies.DefaultInterval,
		Restart:           c.RestartSettings(),
		KillSwitch:        c.KillSwitchSettings(),
		Persistence:       c.PersistenceSettings(),
	}
}

// WithPersistence 注入本地持久化存储（订单、策略状态、快照与日志）。
func WithPersistence(store ports.PersistenceStore) EngineOption {
	return func(e *Engine) {
		e.store = store
	}
}

// WithRiskRules 设置风控规则。
func WithRiskRules(rules risk.Rules) EngineOption {
	return func(e *Engine) {
		if e == nil {
			return
		}
		e.riskEvaluator = NewStaticRiskEvaluator(rules)
	}
}
