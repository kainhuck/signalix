package strategy

import (
	"context"
	"time"

	"github.com/kainhuck/signalix/internal/models"
)

// StrategyRuntime 策略子进程抽象，便于测试注入 fake。
type StrategyRuntime interface {
	Name() string
	Context() context.Context

	SendInit(strategy *Strategy) error
	SendTick(ticker *models.Ticker, traceID string) error
	SendKline(kline *models.Kline, traceID string) error
	SendHistory(payload *models.HistoryPayload) error
	SendStop() error
	ReadMessages(handler func(IpcMessage)) error
	ReadStderr()
	Stop() error

	UpdateHeartbeat()
	GetLastHeartbeat() time.Time
	PruneCrashes(window time.Duration)
	RecordCrash()
	GetCrashCount() int
	IsRunning() bool

	SendRPCResponse(requestID string, result interface{}, err error) error
}
