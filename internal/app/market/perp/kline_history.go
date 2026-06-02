package perp

import (
	"context"
	"sort"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/logger"
)

// DefaultKlineHistoryMax 单 (contract, interval) 缓冲上限（与 Gate REST 上限一致）。
const DefaultKlineHistoryMax = 2000

// WarmupHistory 拉取 REST 历史 K 线并灌入缓冲，返回中性 HistoryPayload（满足 market.MarketFeed）。
// bars <= 0 或无可用数据时返回 (nil, nil)。
func (mr *MarketRouter) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	if bars <= 0 {
		return nil, nil
	}
	series := make([]*models.KlineSeries, 0, len(req.Symbols))
	for _, sym := range req.Symbols {
		contract := perp.Contract(sym)
		snaps, err := mr.exchange.ListCandlesticks(ctx, &perp.ListCandlesticksQuery{
			Contract: contract,
			Interval: req.Interval,
			Limit:    bars,
		})
		if err != nil {
			logger.ErrorContext(ctx, "list candlesticks for history failed",
				logger.String("strategy", req.Strategy),
				logger.String("contract", string(contract)),
				logger.String("interval", req.Interval),
				logger.Any("error", err))
			continue
		}
		barList := models.KlinesFromSnapshots(snaps)
		if len(barList) == 0 {
			continue
		}
		series = append(series, &models.KlineSeries{
			Contract: contract,
			Bars:     barList,
		})
	}
	if len(series) == 0 {
		return nil, nil
	}
	mr.IngestHistoryKlines(req.Interval, series)
	return &models.HistoryPayload{Interval: req.Interval, Series: series}, nil
}

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
