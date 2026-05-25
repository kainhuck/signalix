package models

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// Ticker 与 perp.TickerSnapshot JSON 对齐，用于 IPC 嵌套载荷。
type Ticker struct {
	Contract        perp.Contract `json:"contract"`
	Last            string        `json:"last"`
	MarkPrice       string        `json:"mark_price"`
	IndexPrice      string        `json:"index_price"`
	FundingRate     string        `json:"funding_rate"`
	ChangePct24h    string        `json:"change_pct_24h"`
	Volume24h       string        `json:"volume_24h"`
	Volume24hBase   string        `json:"volume_24h_base"`
	Volume24hQuote  string        `json:"volume_24h_quote"`
	OpenInterest    string        `json:"open_interest"`
	Low24h          string        `json:"low_24h"`
	High24h         string        `json:"high_24h"`
	TimestampMillis int64         `json:"timestamp_millis"`
}

// TickerFromSnapshot 由交易所 ticker 快照构造 IPC 载荷。
func TickerFromSnapshot(snap *perp.TickerSnapshot) *Ticker {
	if snap == nil {
		return nil
	}
	return &Ticker{
		Contract:        snap.Contract,
		Last:            snap.Last,
		MarkPrice:       snap.MarkPrice,
		IndexPrice:      snap.IndexPrice,
		FundingRate:     snap.FundingRate,
		ChangePct24h:    snap.ChangePct24h,
		Volume24h:       snap.Volume24h,
		Volume24hBase:   snap.Volume24hBase,
		Volume24hQuote:  snap.Volume24hQuote,
		OpenInterest:    snap.OpenInterest,
		Low24h:          snap.Low24h,
		High24h:         snap.High24h,
		TimestampMillis: snap.TimestampMillis,
	}
}
