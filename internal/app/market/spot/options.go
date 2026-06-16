package spot

const (
	DefaultMarketBuffer    = 1000
	DefaultKlineHistoryMax = 2000
)

// Option 配置 spot market/router。
type Option func(*SpotMarket)

// WithMarketBuffer 设置 spot market update channel 容量。
func WithMarketBuffer(n int) Option {
	return func(m *SpotMarket) {
		if m != nil && n > 0 {
			m.marketBuf = n
		}
	}
}

// WithKlineHistoryMax 设置单 pair/interval 的历史 K 线缓存上限。
func WithKlineHistoryMax(n int) Option {
	return func(m *SpotMarket) {
		if m != nil && n > 0 {
			m.klineHistoryMax = n
		}
	}
}
