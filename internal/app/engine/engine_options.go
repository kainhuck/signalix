package engine

import (
	"time"

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
}

// BuildParamsFromConfig 从全局配置提取构建参数。
func BuildParamsFromConfig(c *config.Config) BuildParams {
	if c == nil {
		return BuildParams{
			ProjectionRefresh: 10 * time.Second,
			DecisionDivisor:   10,
			OMSMaxRetries:     3,
		}
	}
	return BuildParams{
		Channels:          c.Channels,
		ProjectionRefresh: c.ProjectionRefreshInterval(),
		DecisionDivisor:   c.Decision.DefaultSizeDivisor,
		OMSMaxRetries:     c.Exchange.MaxRetries,
		DefaultInterval:   c.Strategies.DefaultInterval,
	}
}

// WithPersistence 注入本地订单与策略状态存储。
func WithPersistence(store ports.OrderStore) EngineOption {
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
