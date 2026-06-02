package perp

import "github.com/kainhuck/signalix/internal/app/market"

// ErrMarketRouterNotConfigured 表示行情路由器未配置。
var ErrMarketRouterNotConfigured = market.ErrMarketRouterNotConfigured

// ErrTickerNotInCache 表示 ticker 不在 MarketRouter 缓存中。
var ErrTickerNotInCache = market.ErrTickerNotInCache
