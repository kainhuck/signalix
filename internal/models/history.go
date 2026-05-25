package models

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// KlineSeries 单合约历史 K 线序列。
type KlineSeries struct {
	Contract perp.Contract `json:"contract"`
	Bars     []*Kline      `json:"bars"`
}

// HistoryPayload REST 预热 IPC 载荷。
type HistoryPayload struct {
	Interval string         `json:"interval"`
	Series   []*KlineSeries `json:"series"`
}

// KlinesFromSnapshots 将 REST 快照转为 IPC K 线（强制 window_closed=true）。
func KlinesFromSnapshots(snaps []*perp.CandlestickSnapshot) []*Kline {
	if len(snaps) == 0 {
		return nil
	}
	out := make([]*Kline, 0, len(snaps))
	for _, snap := range snaps {
		if snap == nil {
			continue
		}
		k := KlineFromSnapshot(snap)
		if k == nil {
			continue
		}
		k.WindowClosed = true
		out = append(out, k)
	}
	return out
}
