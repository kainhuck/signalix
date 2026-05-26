package grpc

import (
	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
)

func healthReportToProto(report engine.HealthReport) *enginev1.GetHealthReply {
	checks := make([]*enginev1.HealthCheck, 0, len(report.Checks))
	for _, c := range report.Checks {
		checks = append(checks, &enginev1.HealthCheck{
			Name:    c.Name,
			Status:  componentStatusToProto(c.Status),
			Message: c.Message,
		})
	}
	return &enginev1.GetHealthReply{
		Status:           healthStatusToProto(report.Status),
		ServerTimeUnixMs: report.ServerTime.UnixMilli(),
		Checks:           checks,
		KillSwitchActive: report.KillSwitchActive,
	}
}

func healthStatusToProto(s engine.HealthStatus) enginev1.HealthStatus {
	switch s {
	case engine.HealthStatusHealthy:
		return enginev1.HealthStatus_HEALTH_STATUS_HEALTHY
	case engine.HealthStatusDegraded:
		return enginev1.HealthStatus_HEALTH_STATUS_DEGRADED
	case engine.HealthStatusUnhealthy:
		return enginev1.HealthStatus_HEALTH_STATUS_UNHEALTHY
	default:
		return enginev1.HealthStatus_HEALTH_STATUS_UNSPECIFIED
	}
}

func componentStatusToProto(s engine.ComponentStatus) enginev1.ComponentStatus {
	switch s {
	case engine.ComponentStatusPass:
		return enginev1.ComponentStatus_COMPONENT_STATUS_PASS
	case engine.ComponentStatusFail:
		return enginev1.ComponentStatus_COMPONENT_STATUS_FAIL
	default:
		return enginev1.ComponentStatus_COMPONENT_STATUS_UNSPECIFIED
	}
}
