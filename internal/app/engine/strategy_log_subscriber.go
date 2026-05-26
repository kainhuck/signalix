package engine

import (
	"context"
	"sync/atomic"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/logger"
)

type strategyLogSubscription struct {
	ch             chan *models.StrategyLogRow
	strategyName   string
	minExclusiveID int64
}

// PersistenceStore 返回持久化 store（可能为 nil）。
func (e *Engine) PersistenceStore() ports.PersistenceStore {
	if e == nil {
		return nil
	}
	return e.store
}

// RegisterStrategyLogSubscriber 注册策略日志流订阅；ctx 取消时注销并 close channel。
func (e *Engine) RegisterStrategyLogSubscriber(
	ctx context.Context,
	strategyName string,
	minExclusiveID int64,
	buf int,
) <-chan *models.StrategyLogRow {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan *models.StrategyLogRow, buf)
	if e == nil {
		close(ch)
		return ch
	}
	id := atomic.AddUint64(&e.strategyLogSubNext, 1)
	sub := &strategyLogSubscription{
		ch:             ch,
		strategyName:   strategyName,
		minExclusiveID: minExclusiveID,
	}
	e.strategyLogSubMu.Lock()
	if e.strategyLogSubs == nil {
		e.strategyLogSubs = make(map[uint64]*strategyLogSubscription)
	}
	e.strategyLogSubs[id] = sub
	e.strategyLogSubMu.Unlock()
	go func() {
		<-ctx.Done()
		e.strategyLogSubMu.Lock()
		delete(e.strategyLogSubs, id)
		close(ch)
		e.strategyLogSubMu.Unlock()
	}()
	return ch
}

func (e *Engine) notifyStrategyLogSubscribers(row *models.StrategyLogRow) {
	if e == nil || row == nil || row.ID <= 0 {
		return
	}
	e.strategyLogSubMu.RLock()
	defer e.strategyLogSubMu.RUnlock()
	for id, sub := range e.strategyLogSubs {
		if sub == nil {
			continue
		}
		if sub.strategyName != "" && sub.strategyName != row.StrategyName {
			continue
		}
		if row.ID <= sub.minExclusiveID {
			continue
		}
		select {
		case sub.ch <- row:
		default:
			logger.WarnContext(e.ctx, "strategy log grpc subscriber channel full",
				logger.Uint64("subscriber_id", id),
				logger.Int64("log_id", row.ID))
		}
	}
}
