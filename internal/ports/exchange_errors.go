package ports

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// IsOrderNotFound 判断是否为交易所「订单不存在」类错误。
func IsOrderNotFound(err error) bool {
	return perp.IsOrderNotFound(err)
}
