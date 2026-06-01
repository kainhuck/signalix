package market

import (
	"context"

	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// MarketRouter 实现 perp 的 MarketFeed。
var _ MarketFeed = (*MarketRouter)(nil)

// Subscribe 满足 MarketFeed：将中性 SubscribeRequest 转为 perp 合约后订阅。
func (mr *MarketRouter) Subscribe(ctx context.Context, req SubscribeRequest) error {
	_ = ctx // 当前 WS 订阅沿用 mr.ctx；ctx 预留未来取消用
	return mr.SubscribeContracts(req.Strategy, contractsFromStrings(req.Symbols), req.Interval, req.PushTicker)
}

// Unsubscribe 满足 MarketFeed：取消该策略在本市场的全部订阅。
func (mr *MarketRouter) Unsubscribe(strategy string) error {
	return mr.UnsubscribeAll(strategy)
}

// Updates 满足 MarketFeed：归一化行情流。
func (mr *MarketRouter) Updates() <-chan MarketUpdate {
	return mr.marketCh
}

func contractsFromStrings(ss []string) []perp.Contract {
	out := make([]perp.Contract, 0, len(ss))
	for _, s := range ss {
		out = append(out, perp.Contract(s))
	}
	return out
}
