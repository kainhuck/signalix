package spot

import (
	"context"
	"sort"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	spotex "github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/kainhuck/signalix/pkg/logger"
)

func (mr *MarketRouter) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	if bars <= 0 {
		return nil, nil
	}
	pairs, err := canonicalPairs(req.Symbols)
	if err != nil {
		return nil, err
	}
	series := make([]*models.KlineSeries, 0, len(pairs))
	for _, pair := range pairs {
		snaps, err := mr.exchange.ListCandlesticks(ctx, &spotex.ListCandlesticksQuery{
			Pair:     pair,
			Interval: req.Interval,
			Limit:    bars,
		})
		if err != nil {
			logger.ErrorContext(ctx, "list spot candlesticks for history failed",
				logger.String("strategy", req.Strategy),
				logger.String("pair", pair.String()),
				logger.String("interval", req.Interval),
				logger.Any("error", err))
			continue
		}
		bars := klinesFromSpot(snaps)
		if len(bars) == 0 {
			continue
		}
		series = append(series, &models.KlineSeries{
			Contract: bars[0].Contract,
			Bars:     bars,
		})
	}
	if len(series) == 0 {
		return nil, nil
	}
	mr.IngestHistoryKlines(req.Interval, series)
	return &models.HistoryPayload{Interval: req.Interval, Series: series}, nil
}

func klinesFromSpot(snaps []*spotex.CandlestickSnapshot) []*models.Kline {
	if len(snaps) == 0 {
		return nil
	}
	out := make([]*models.Kline, 0, len(snaps))
	for _, snap := range snaps {
		k := klineFromSpot(snap)
		if k == nil {
			continue
		}
		k.WindowClosed = true
		out = append(out, k)
	}
	return out
}

func (mr *MarketRouter) appendClosedKline(snap *spotex.CandlestickSnapshot) {
	if snap == nil || !snap.WindowClosed {
		return
	}
	k := klineFromSpot(snap)
	if k == nil {
		return
	}
	key := candleKey{Pair: snap.Pair.Canonical(), Interval: snap.Interval}
	mr.klineHistMu.Lock()
	defer mr.klineHistMu.Unlock()
	mr.appendKlineLocked(key, k)
}

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
		key := candleKey{Pair: spotex.CanonicalPair(string(s.Contract)), Interval: interval}
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

func (mr *MarketRouter) ListClosedKlines(pair spotex.Pair, interval string, limit int) []*models.Kline {
	if pair == "" || interval == "" || limit <= 0 {
		return nil
	}
	key := candleKey{Pair: pair.Canonical(), Interval: interval}
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
