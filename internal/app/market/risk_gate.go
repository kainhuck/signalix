package market

// NotionalGate 由 Engine 实现，供各市场 Risk 判断是否需要计算名义字段。
type NotionalGate interface {
	NeedsNotional(increasingExposure bool) bool
}
