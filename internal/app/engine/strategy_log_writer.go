package engine

import (
	"context"
	"strings"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/logger"
)

const strategyLogQueueSize = 512

func (e *Engine) startStrategyLogWriter() {
	if e == nil || e.store == nil || e.strategyLogCh != nil {
		return
	}
	e.strategyLogCh = make(chan *models.StrategyLogRow, strategyLogQueueSize)
	e.wg.Add(1)
	go e.strategyLogWriterLoop()
}

func (e *Engine) stopStrategyLogWriter() {
	if e == nil || e.strategyLogCh == nil {
		return
	}
	e.strategyLogStopOnce.Do(func() {
		close(e.strategyLogCh)
	})
}

func (e *Engine) enqueueStrategyLog(strategyName, level, message string) {
	if e == nil || e.store == nil || e.strategyLogCh == nil {
		return
	}
	if strings.TrimSpace(message) == "" {
		return
	}
	row := &models.StrategyLogRow{
		StrategyName: strategyName,
		Level:        level,
		Message:      message,
		CreatedAt:    time.Now().UTC(),
	}
	select {
	case e.strategyLogCh <- row:
	default:
		logger.WarnContext(e.ctx, "strategy log queue full, dropping",
			logger.String("strategy", strategyName))
	}
}

func (e *Engine) strategyLogWriterLoop() {
	defer e.wg.Done()
	for row := range e.strategyLogCh {
		if row == nil {
			continue
		}
		if err := e.store.SaveStrategyLog(context.Background(), row); err != nil {
			logger.WarnContext(e.ctx, "strategy log persist failed",
				logger.String("strategy", row.StrategyName),
				logger.String("level", row.Level),
				logger.Any("error", err))
			continue
		}
		e.notifyStrategyLogSubscribers(row)
	}
}
