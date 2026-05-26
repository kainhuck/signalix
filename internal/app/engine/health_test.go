package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/internal/testutil"
)

type pingFailExchange struct {
	testutil.StubExchange
}

func (p *pingFailExchange) Ping(context.Context) error {
	return errors.New("ping fail")
}

func newEngineForHealthTest(t *testing.T, ex ports.Exchange, running, startProjection bool) *Engine {
	t.Helper()
	e := NewEngine(t.TempDir(), ex, BuildParams{})
	if startProjection {
		if err := e.accountProjection.Start(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if running {
		e.running.Store(true)
	}
	return e
}

func TestHealthReport_engineNotRunning(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, testutil.NewStubExchange(), false, true)
	report := e.HealthReport(t.Context(), true)
	if report.Status != HealthStatusUnhealthy {
		t.Fatalf("status = %v, want UNHEALTHY", report.Status)
	}
	if checkStatus(report.Checks, healthCheckEngine) != ComponentStatusFail {
		t.Fatal("expected engine FAIL")
	}
}

func TestHealthReport_projectionNotReady(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, testutil.NewStubExchange(), true, false)
	report := e.HealthReport(t.Context(), true)
	if report.Status != HealthStatusUnhealthy {
		t.Fatalf("status = %v, want UNHEALTHY", report.Status)
	}
	if checkStatus(report.Checks, healthCheckAccountProjection) != ComponentStatusFail {
		t.Fatal("expected account_projection FAIL")
	}
}

func TestHealthReport_exchangeFail(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, &pingFailExchange{}, true, true)
	report := e.HealthReport(t.Context(), false)
	if report.Status != HealthStatusDegraded {
		t.Fatalf("status = %v, want DEGRADED", report.Status)
	}
	if checkStatus(report.Checks, healthCheckExchange) != ComponentStatusFail {
		t.Fatal("expected exchange FAIL")
	}
}

func TestHealthReport_killSwitchActive(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, testutil.NewStubExchange(), true, true)
	if _, err := e.ActivateKillSwitch(t.Context(), "test", false); err != nil {
		t.Fatal(err)
	}
	report := e.HealthReport(t.Context(), false)
	if report.Status != HealthStatusDegraded {
		t.Fatalf("status = %v, want DEGRADED", report.Status)
	}
	if !report.KillSwitchActive {
		t.Fatal("expected kill_switch_active")
	}
}

func TestHealthReport_allPass(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, testutil.NewStubExchange(), true, true)
	report := e.HealthReport(t.Context(), false)
	if report.Status != HealthStatusHealthy {
		t.Fatalf("status = %v, want HEALTHY", report.Status)
	}
	if report.KillSwitchActive {
		t.Fatal("kill switch should be inactive")
	}
}

func TestHealthReport_skipExchangePing(t *testing.T) {
	t.Parallel()
	e := newEngineForHealthTest(t, &pingFailExchange{}, true, true)
	report := e.HealthReport(t.Context(), true)
	if report.Status != HealthStatusHealthy {
		t.Fatalf("status = %v, want HEALTHY when exchange skipped", report.Status)
	}
	for _, c := range report.Checks {
		if c.Name == healthCheckExchange {
			t.Fatal("exchange check should be omitted")
		}
	}
}

func checkStatus(checks []HealthCheckResult, name string) ComponentStatus {
	for _, c := range checks {
		if c.Name == name {
			return c.Status
		}
	}
	return ComponentStatusUnspecified
}
