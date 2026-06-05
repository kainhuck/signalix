package spot

import (
	"context"
	"sort"

	"github.com/kainhuck/signalix/internal/app/market"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"github.com/kainhuck/signalix/pkg/exchange/spot"
	"github.com/kainhuck/signalix/pkg/logger"
)

func (sr *SpotRouter) WarmupHistory(ctx context.Context, req market.SubscribeRequest, bars int) (*models.HistoryPayload, error) {
	if bars <= 0 {
		return nil, nil
	}
	series := make([]*models.KlineSeries, 0, len(req.Symbols))
	for _, sym := range req.Symbols {
		pair := spot.CanonicalPair(sym)
		snaps, err := sr.client.ListCandlesticks(ctx, &spot.ListCandlesticksQuery{
			Pair:     pair,
			Interval: req.Interval,
			Limit:    bars,
		})
		if err != nil {
			logger.ErrorContext(ctx, "spot list candlesticks for history failed",
				logger.String("strategy", req.Strategy),
				logger.String("pair", string(pair)),
				logger.String("interval", req.Interval),
				logger.Any("error", err))
			continue
		}
		barList := SpotKlinesFromSnapshots(snaps)
		if len(barList) == 0 {
			continue
		}
		series = append(series, &models.KlineSeries{
			Contract: perp.Contract(string(pair)),
			Bars:     barList,
		})
	}
	if len(series) == 0 {
		return nil, nil
	}
	sr.IngestHistoryKlines(req.Interval, series)
	return &models.HistoryPayload{Interval: req.Interval, Series: series}, nil
}

func (sr *SpotRouter) appendClosedKline(snap *spot.CandlestickSnapshot) {
	if snap == nil || !snap.WindowClosed {
		return
	}
	k := spotKlineToModel(snap)
	if k == nil {
		return
	}
	key := pairCandleKey{Pair: snap.Pair, Interval: snap.Interval}
	sr.klineHistMu.Lock()
	defer sr.klineHistMu.Unlock()
	sr.appendKlineLocked(key, k)
}

func (sr *SpotRouter) IngestHistoryKlines(interval string, series []*models.KlineSeries) {
	if interval == "" || len(series) == 0 {
		return
	}
	sr.klineHistMu.Lock()
	defer sr.klineHistMu.Unlock()
	for _, s := range series {
		if s == nil || s.Contract == "" || len(s.Bars) == 0 {
			continue
		}
		key := pairCandleKey{Pair: spot.Pair(s.Contract), Interval: interval}
		for _, bar := range s.Bars {
			if bar == nil {
				continue
			}
			k := *bar
			k.WindowClosed = true
			if k.Interval == "" {
				k.Interval = interval
			}
			sr.mergeKlineLocked(key, &k)
		}
		sr.sortTrimLocked(key)
	}
}

func (sr *SpotRouter) ListClosedKlines(pair spot.Pair, interval string, limit int) []*models.Kline {
	if pair == "" || interval == "" || limit <= 0 {
		return nil
	}
	key := pairCandleKey{Pair: pair, Interval: interval}
	max := sr.klineHistMax
	if max <= 0 {
		max = defaultKlineHistoryMax
	}
	if limit > max {
		limit = max
	}

	sr.klineHistMu.RLock()
	defer sr.klineHistMu.RUnlock()
	list := sr.klineHistory[key]
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

func (sr *SpotRouter) appendKlineLocked(key pairCandleKey, k *models.Kline) {
	list := sr.klineHistory[key]
	if len(list) > 0 && list[len(list)-1].TimestampSec == k.TimestampSec {
		list[len(list)-1] = k
	} else {
		list = append(list, k)
	}
	sr.klineHistory[key] = sr.trimList(list)
}

func (sr *SpotRouter) mergeKlineLocked(key pairCandleKey, k *models.Kline) {
	list := sr.klineHistory[key]
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
	sr.klineHistory[key] = list
}

func (sr *SpotRouter) sortTrimLocked(key pairCandleKey) {
	list := sr.klineHistory[key]
	if len(list) == 0 {
		return
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TimestampSec < list[j].TimestampSec
	})
	sr.klineHistory[key] = sr.trimList(list)
}

func (sr *SpotRouter) trimList(list []*models.Kline) []*models.Kline {
	max := sr.klineHistMax
	if max <= 0 {
		max = defaultKlineHistoryMax
	}
	if len(list) <= max {
		return list
	}
	return list[len(list)-max:]
}

func SpotKlinesFromSnapshots(snaps []*spot.CandlestickSnapshot) []*models.Kline {
	if len(snaps) == 0 {
		return nil
	}
	out := make([]*models.Kline, 0, len(snaps))
	for _, snap := range snaps {
		if snap == nil {
			continue
		}
		k := spotKlineToModel(snap)
		if k == nil {
			continue
		}
		k.WindowClosed = true
		out = append(out, k)
	}
	return out
}
