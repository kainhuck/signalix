package oms

import (
	"context"

	"github.com/kainhuck/signalix/internal/models"
)

// OMS 命令类型（单 goroutine 内处理）。
const (
	omsOpSubmit = iota
	omsOpCancel
	omsOpSync
	omsOpFlush
)

// omsCmd 投递到 ExecutionEngine.cmdCh，由 run() 串行处理。
type omsCmd struct {
	op int

	order *models.Order // Submit

	orderID string          // Cancel / Sync
	ctx     context.Context // Cancel / Sync / Flush 超时控制
	reply   chan error      // buffer 1；Submit/Cancel/Sync/Flush 均用于同步返回
}
