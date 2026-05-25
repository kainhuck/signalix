package market

import (
	"sort"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
)

// DefaultKlineHistoryMax 单 (contract, interval) 缓冲上限（与 Gate REST 上限一致）。
const DefaultKlineHistoryMax = 2000

func (mr *MarketRouter) appendClosedKline(snap *perp.CandlestickSnapshot) {
	if snap == nil || !snap.WindowClosed {
		return
	}
	k := models.KlineFromSnapshot(snap)
	if k == nil {
		return
	}
	key := candleKey{Contract: snap.Contract, Interval: snap.Interval}
	mr.klineHistMu.Lock()
	defer mr.klineHistMu.Unlock()
	mr.appendKlineLocked(key, k)
}

// IngestHistoryKlines 将 REST 预热序列写入缓冲（按 timestamp_sec 合并，升序）。
func (mr *MarketRouter) IngestHistoryKlines(interval string, series []*models.KlineSeries) {
	if interval == "" || len(series) == 0 {
		return
	}
	mr.klineHistMu.Lock()
	defer mr.klineHistMu.Unlock()
	for _, s := range series {
		if s == nil || s.Contract == "" || len(s.Bars) == 0 {
			continue
		}
		key := candleKey{Contract: s.Contract, Interval: interval}
		for _, bar := range s.Bars {
			if bar == nil {
				continue
			}
			k := *bar
			k.WindowClosed = true
			if k.Interval == "" {
				k.Interval = interval
			}
			mr.mergeKlineLocked(key, &k)
		}
		mr.sortTrimLocked(key)
	}
}

// ListClosedKlines 返回最近 limit 根收盘 K 线（时间升序）。
func (mr *MarketRouter) ListClosedKlines(contract perp.Contract, interval string, limit int) []*models.Kline {
	if contract == "" || interval == "" || limit <= 0 {
		return nil
	}
	key := candleKey{Contract: contract, Interval: interval}
	max := mr.klineHistMax
	if max <= 0 {
		max = DefaultKlineHistoryMax
	}
	if limit > max {
		limit = max
	}

	mr.klineHistMu.RLock()
	defer mr.klineHistMu.RUnlock()
	list := mr.klineHistory[key]
	if len(list) == 0 {
		return nil
	}
	start := 0
	if len(list) > limit {
		start = len(list) - limit
	}
	out := make([]*models.Kline, 0, limit)
	for _, k := range list[start:] {
		if k == nil {
			continue
		}
		cp := *k
		out = append(out, &cp)
	}
	return out
}

func (mr *MarketRouter) appendKlineLocked(key candleKey, k *models.Kline) {
	list := mr.klineHistory[key]
	if len(list) > 0 && list[len(list)-1].TimestampSec == k.TimestampSec {
		list[len(list)-1] = k
	} else {
		list = append(list, k)
	}
	mr.klineHistory[key] = mr.trimList(list)
}

func (mr *MarketRouter) mergeKlineLocked(key candleKey, k *models.Kline) {
	list := mr.klineHistory[key]
	idx := sort.Search(len(list), func(i int) bool {
		return list[i].TimestampSec >= k.TimestampSec
	})
	if idx < len(list) && list[idx].TimestampSec == k.TimestampSec {
		list[idx] = k
	} else {
		list = append(list, nil)
		copy(list[idx+1:], list[idx:])
		list[idx] = k
	}
	mr.klineHistory[key] = list
}

func (mr *MarketRouter) sortTrimLocked(key candleKey) {
	list := mr.klineHistory[key]
	if len(list) == 0 {
		return
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TimestampSec < list[j].TimestampSec
	})
	mr.klineHistory[key] = mr.trimList(list)
}

func (mr *MarketRouter) trimList(list []*models.Kline) []*models.Kline {
	max := mr.klineHistMax
	if max <= 0 {
		max = DefaultKlineHistoryMax
	}
	if len(list) <= max {
		return list
	}
	return list[len(list)-max:]
}
