package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

func (e *Engine) loadSnapshotAndReconcile() error {
	if e.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, 45*time.Second)
	defer cancel()

	rawStates, err := e.store.LoadAllStrategyStates(ctx)
	if err != nil {
		return fmt.Errorf("load strategy states: %w", err)
	}

	e.strategyStateMu.Lock()
	for sn, m := range rawStates {
		if e.strategyStates[sn] == nil {
			e.strategyStates[sn] = make(map[string]interface{})
		}
		for k, raw := range m {
			var v interface{}
			if err := json.Unmarshal(raw, &v); err != nil {
				logger.WarnContext(e.ctx, "skip invalid strategy_state json",
					logger.String("strategy", sn),
					logger.String("key", k),
					logger.Any("error", err))
				continue
			}
			e.strategyStates[sn][k] = v
		}
	}
	e.strategyStateMu.Unlock()

	orders, err := e.store.ListNonTerminalOrders(ctx)
	if err != nil {
		return fmt.Errorf("list non-terminal orders: %w", err)
	}

	reconcileWithExchange(ctx, e.exchange, e.store, orders)
	e.executionEngine.HydrateFromSnapshot(orders)
	return nil
}

func reconcileWithExchange(ctx context.Context, ex ports.Exchange, store ports.OrderStore, orders []*models.Order) {
	if store == nil || ex == nil {
		return
	}
	for _, o := range orders {
		if o == nil {
			continue
		}
		switch o.Status {
		case models.OrderStatusFilled, models.OrderStatusCancelled, models.OrderStatusRejected:
			continue
		}
		if o.ExchangeID == "" {
			logger.InfoContext(ctx, "reconcile skip pending order without exchange_id",
				logger.String("order_id", o.ID),
				logger.String("strategy", o.StrategyName),
				logger.String("contract", string(o.Symbol)))
			continue
		}

		ev, err := ex.GetOrder(ctx, o.Symbol, o.ExchangeID)
		if err != nil {
			if perp.IsOrderNotFound(err) {
				prev := o.Status
				o.Status = models.OrderStatusCancelled
				o.UpdatedAt = time.Now()
				if err := store.SaveOrder(ctx, o); err != nil {
					logger.ErrorContext(ctx, "reconcile persist cancelled order failed",
						logger.String("order_id", o.ID),
						logger.Any("error", err))
				} else {
					logger.WarnContext(ctx, "reconcile order not found on exchange, marked cancelled",
						logger.String("order_id", o.ID),
						logger.String("exchange_id", o.ExchangeID),
						logger.String("contract", string(o.Symbol)),
						logger.String("previous_status", string(prev)))
				}
				continue
			}
			logger.WarnContext(ctx, "reconcile get order failed, continuing startup",
				logger.String("order_id", o.ID),
				logger.String("exchange_id", o.ExchangeID),
				logger.String("contract", string(o.Symbol)),
				logger.Any("error", err))
			continue
		}

		newStatus := models.OrderStatus(ev.Status)
		if o.Status != newStatus || o.FilledSize != ev.FilledSize {
			logger.WarnContext(ctx, "reconcile order state differs from exchange, applying exchange view",
				logger.String("order_id", o.ID),
				logger.String("exchange_id", o.ExchangeID),
				logger.String("contract", string(o.Symbol)),
				logger.String("local_status", string(o.Status)),
				logger.String("exchange_status", string(ev.Status)),
				logger.String("local_filled", o.FilledSize),
				logger.String("exchange_filled", ev.FilledSize))
		}
		o.Status = newStatus
		o.FilledSize = ev.FilledSize
		o.UpdatedAt = ev.UpdatedAt
		if err := store.SaveOrder(ctx, o); err != nil {
			logger.ErrorContext(ctx, "reconcile persist order failed",
				logger.String("order_id", o.ID),
				logger.Any("error", err))
		}
	}
}
