package gateio

import (
	"context"
	"strconv"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/spot"

	"github.com/antihax/optional"
	"github.com/gate/gateapi-go/v7"
)

const maxSpotCandlesticksPerRequest = 1000

// ListCandlesticks 拉取现货历史 K 线（公开 REST）。
func (c *Client) ListCandlesticks(ctx context.Context, q *spot.ListCandlesticksQuery) ([]*spot.CandlestickSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	if q == nil {
		return nil, spot.NewError(spot.ErrInvalidParameter, "query is nil", nil)
	}
	pair := strings.TrimSpace(string(q.Pair.Canonical()))
	if pair == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, "pair is required", nil)
	}
	interval := strings.TrimSpace(q.Interval)
	if interval == "" {
		return nil, spot.NewError(spot.ErrInvalidParameter, "interval is required", nil)
	}

	gatePair := toGatePair(spot.Pair(pair))
	opts := &gateapi.ListCandlesticksOpts{
		Interval: optional.NewString(interval),
	}

	hasFrom := q.From > 0
	hasTo := q.To > 0
	if hasFrom || hasTo {
		if q.Limit > 0 {
			return nil, spot.NewError(spot.ErrInvalidParameter, "limit cannot be used together with from/to", nil)
		}
		if hasFrom {
			opts.From = optional.NewInt64(q.From)
		}
		if hasTo {
			opts.To = optional.NewInt64(q.To)
		}
	} else {
		limit := q.Limit
		if limit <= 0 {
			limit = 100
		}
		if limit > maxSpotCandlesticksPerRequest {
			return nil, spot.NewError(spot.ErrInvalidParameter, "limit exceeds 1000", nil)
		}
		opts.Limit = optional.NewInt32(int32(limit))
	}

	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	rows, _, err := c.gate.SpotApi.ListCandlesticks(c.publicCtx(ctx), gatePair, opts)
	if err != nil {
		return nil, mapGateAPIError(err)
	}

	out := make([]*spot.CandlestickSnapshot, 0, len(rows))
	for _, row := range rows {
		snap := candlestickFromREST(gatePair, interval, row)
		if snap != nil {
			out = append(out, snap)
		}
	}
	return out, nil
}

// Gate spot candlestick row: [timestamp, quote_volume, close, high, low, open]
func candlestickFromREST(gatePair, interval string, row []string) *spot.CandlestickSnapshot {
	if len(row) < 6 {
		return nil
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
	if err != nil || ts <= 0 {
		return nil
	}
	return &spot.CandlestickSnapshot{
		Pair:         spot.CanonicalPair(gatePair),
		Interval:     interval,
		Volume:       strings.TrimSpace(row[1]),
		Close:        strings.TrimSpace(row[2]),
		High:         strings.TrimSpace(row[3]),
		Low:          strings.TrimSpace(row[4]),
		Open:         strings.TrimSpace(row[5]),
		TimestampSec: ts,
		WindowClosed: true,
	}
}
