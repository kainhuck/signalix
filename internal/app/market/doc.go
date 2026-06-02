// Package market 定义引擎对"一个已接入市场"的统一抽象（Market）及其按职责拆分的
// 小接口（MarketFeed / MarketDecider / MarketExecutor / MarketRisk / MarketAccount）。
//
// 设计意图：引擎层不关心接入的是 perp 还是 spot，只面向 Market 接口编排，并允许接入
// 多个市场（引擎持有 map[models.Market]Market）。perp/spot 各自在子包实现该抽象，
// 取代散落在引擎内部的 `if market == spot/perp` 分支。
//
// 接口约束：
//   - 所有方法签名只使用 internal/models 与 internal/ports 的市场中性类型；
//     禁止 perp.* / spot.* 等具体交易所类型出现在接口签名中。
//   - MarketExecutor 仅做无状态翻译（中性 Order -> 所侧请求、Place/Cancel/Sync），
//     不持有/不修改订单状态机——订单状态机仍由 OMS 单写者集中管理。
//
// 具体实现：
//   - perp：[perp/](./perp/)（PerpMarket、MarketRouter、Executor 等）
package market
