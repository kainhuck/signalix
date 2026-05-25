package models

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// Kline 发送给策略脚本的收盘 K 线（与 perp.CandlestickSnapshot JSON 对齐）。
type Kline struct {
	Contract     perp.Contract `json:"contract"`
	Interval     string        `json:"interval"`
	Open         string        `json:"open"`
	High         string        `json:"high"`
	Low          string        `json:"low"`
	Close        string        `json:"close"`
	Volume       string        `json:"volume"`
	VolumeBase   string        `json:"volume_base,omitempty"`
	TimestampSec int64         `json:"timestamp_sec"`
	WindowClosed bool          `json:"window_closed"`
}

// KlineFromSnapshot 由交易所 K 线快照构造 IPC 载荷。
func KlineFromSnapshot(snap *perp.CandlestickSnapshot) *Kline {
	if snap == nil {
		return nil
	}
	return &Kline{
		Contract:     snap.Contract,
		Interval:     snap.Interval,
		Open:         snap.Open,
		High:         snap.High,
		Low:          snap.Low,
		Close:        snap.Close,
		Volume:       snap.Volume,
		VolumeBase:   snap.VolumeBase,
		TimestampSec: snap.TimestampSec,
		WindowClosed: snap.WindowClosed,
	}
}
