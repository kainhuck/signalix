package engine

import (
	"context"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

const (
	healthCheckEngine            = "engine"
	healthCheckAccountProjection = "account_projection"
	healthCheckExchange          = "exchange"

	healthExchangePingTimeout = 2 * time.Second
)

// HealthStatus 聚合就绪状态（映射 gRPC HealthStatus）。
type HealthStatus int

const (
	HealthStatusUnspecified HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// ComponentStatus 单项检查结果。
type ComponentStatus int

const (
	ComponentStatusUnspecified ComponentStatus = iota
	ComponentStatusPass
	ComponentStatusFail
)

// HealthCheckResult 单项健康检查结果。
type HealthCheckResult struct {
	Name    string
	Status  ComponentStatus
	Message string
}

// HealthReport 只读快照，供 gRPC GetHealth 使用。
type HealthReport struct {
	Status           HealthStatus
	ServerTime       time.Time
	Checks           []HealthCheckResult
	KillSwitchActive bool
}

// HealthReport 构建就绪报告。ctx 用于 exchange.Ping 超时继承。
func (e *Engine) HealthReport(ctx context.Context, skipExchangePing bool) HealthReport {
	now := time.Now()
	if e == nil {
		return HealthReport{
			Status:     HealthStatusUnhealthy,
			ServerTime: now,
			Checks: []HealthCheckResult{
				{Name: healthCheckEngine, Status: ComponentStatusFail, Message: "nil engine"},
			},
		}
	}

	checks := make([]HealthCheckResult, 0, 3)

	enginePass := e.Running()
	engineMsg := ""
	if !enginePass {
		engineMsg = "engine not running"
	}
	checks = append(checks, HealthCheckResult{
		Name:    healthCheckEngine,
		Status:  boolToComponentStatus(enginePass),
		Message: engineMsg,
	})

	projPass := false
	projMsg := ""
	if e.accountProjection == nil {
		projMsg = "not configured"
	} else if e.accountProjection.IsReady() {
		projPass = true
	} else {
		projMsg = "not ready"
	}
	checks = append(checks, HealthCheckResult{
		Name:    healthCheckAccountProjection,
		Status:  boolToComponentStatus(projPass),
		Message: projMsg,
	})

	exchangePass := true
	if !skipExchangePing {
		exPass, exMsg := e.checkExchange(ctx)
		exchangePass = exPass
		checks = append(checks, HealthCheckResult{
			Name:    healthCheckExchange,
			Status:  boolToComponentStatus(exPass),
			Message: exMsg,
		})
	}

	ksActive := e.killSwitchActive()
	status := aggregateHealthStatus(enginePass, projPass, exchangePass, ksActive, skipExchangePing)

	return HealthReport{
		Status:           status,
		ServerTime:       now,
		Checks:           checks,
		KillSwitchActive: ksActive,
	}
}

func (e *Engine) checkExchange(ctx context.Context) (pass bool, message string) {
	m := e.marketFor(models.MarketPerp)
	if m == nil {
		return false, "not configured"
	}
	pingCtx, cancel := context.WithTimeout(ctx, healthExchangePingTimeout)
	defer cancel()
	if err := m.Ping(pingCtx); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func boolToComponentStatus(ok bool) ComponentStatus {
	if ok {
		return ComponentStatusPass
	}
	return ComponentStatusFail
}

func aggregateHealthStatus(enginePass, projPass, exchangePass, killSwitchActive, skipExchangePing bool) HealthStatus {
	if !enginePass || !projPass {
		return HealthStatusUnhealthy
	}
	if !skipExchangePing && !exchangePass {
		return HealthStatusDegraded
	}
	if killSwitchActive {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}
