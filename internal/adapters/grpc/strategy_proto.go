package grpc

import (
	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
)

func strategySummaryToProto(s engine.StrategyRuntimeSnapshot) *enginev1.StrategySummary {
	syms := make([]string, 0, len(s.Symbols))
	for _, sym := range s.Symbols {
		syms = append(syms, string(sym))
	}
	po := &enginev1.StrategySummary{
		Name:               s.Name,
		Enabled:            s.Enabled,
		Symbols:            syms,
		Running:            s.Running,
		CrashCount:         int32(s.CrashCount),
		CrashCountInWindow: int32(s.CrashCountInWindow),
		AutoRestartEnabled: s.AutoRestartEnabled,
		RestartBackoffSec:  s.RestartBackoffSec,
		CircuitOpen:        s.CircuitOpen,
	}
	if !s.LastHeartbeat.IsZero() {
		po.LastHeartbeatUnixMs = s.LastHeartbeat.UnixMilli()
	}
	return po
}
