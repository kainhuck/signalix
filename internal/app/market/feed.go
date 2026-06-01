package market

import (
	"context"

	"github.com/kainhuck/signalix/internal/models"
)

// SubscribeRequest 行情订阅请求（市场中性）。
type SubscribeRequest struct {
	Strategy   string   // 策略名（fan-out / 引用计数键）
	Symbols    []string // 中性 symbol 字符串；实现内部 Canonical 到 perp.Contract / spot.Pair
	Interval   string   // K 线周期
	PushTicker bool     // 是否向策略推送 tick（对应 subscribe_ticker）
}

// MarketFeed 行情订阅与归一化分发。
type MarketFeed interface {
	// Subscribe 订阅行情；引擎始终订 ticker 写缓存，PushTicker 为 true 时额外推送 tick。
	Subscribe(ctx context.Context, req SubscribeRequest) error
	// Unsubscribe 取消该策略在本市场的全部订阅。
	Unsubscribe(strategy string) error
	// WarmupHistory 拉取 REST 历史 K 线，返回中性 HistoryPayload；实现内部同时灌入自身缓存。
	WarmupHistory(ctx context.Context, req SubscribeRequest, bars int) (*models.HistoryPayload, error)
	// Updates 归一化后的行情流（ticker / kline）；引擎统一消费，不再 switch market。
	Updates() <-chan MarketUpdate
}
