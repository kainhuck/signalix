package perp

// ClientOrderIDCodec 将引擎本地 Order.ID 与交易所返回的 client order tag 互转。
//
// TagFromLocal：下单时写入所侧（Gate text / Binance clientOrderId 等）。
// LocalFromTag：从所侧 tag 无状态还原本地 ID；不可逆时 ok=false（须配合运行时索引）。
type ClientOrderIDCodec interface {
	TagFromLocal(localID string) string
	LocalFromTag(tag string) (localID string, ok bool)
}
