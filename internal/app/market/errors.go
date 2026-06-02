package market

import "errors"

// ErrMarketRouterNotConfigured 表示行情路由器未配置。
var ErrMarketRouterNotConfigured = errors.New("market router not configured")

// ErrTickerNotInCache 表示 ticker 不在 MarketRouter 缓存中。
var ErrTickerNotInCache = errors.New("ticker not in cache")
