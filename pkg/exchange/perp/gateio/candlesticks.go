package gateio

import (
	"context"
	"strings"

	"github.com/kainhuck/signalix/pkg/exchange/perp"

	"github.com/antihax/optional"
	"github.com/gate/gateapi-go/v7"
)

const maxCandlesticksPerRequest = 2000

// ListCandlesticks 拉取合约历史 K 线（公开 REST，无需鉴权）。
// 见 https://www.gate.com/docs/developers/apiv4/en/futures/#futures-market-k-line-chart
func (c *Client) ListCandlesticks(ctx context.Context, q *perp.ListCandlesticksQuery) ([]*perp.CandlestickSnapshot, error) {
	if err := c.requireREST(); err != nil {
		return nil, err
	}
	if q == nil {
		return nil, perp.NewError(perp.ErrInvalidParameter, "query is nil", nil)
	}
	contract := strings.TrimSpace(string(q.Contract.Canonical()))
	if contract == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, "contract is required", nil)
	}
	interval := strings.TrimSpace(q.Interval)
	if interval == "" {
		return nil, perp.NewError(perp.ErrInvalidParameter, "interval is required", nil)
	}

	gateContract := toGateContract(perp.Contract(contract))
	opts := &gateapi.ListFuturesCandlesticksOpts{
		Interval: optional.NewString(interval),
	}
	if tz := strings.TrimSpace(q.Timezone); tz != "" {
		opts.Timezone = optional.NewString(tz)
	}

	hasFrom := q.From > 0
	hasTo := q.To > 0
	if hasFrom || hasTo {
		if q.Limit > 0 {
			return nil, perp.NewError(perp.ErrInvalidParameter, "limit cannot be used together with from/to", nil)
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
		if limit > maxCandlesticksPerRequest {
			return nil, perp.NewError(perp.ErrInvalidParameter, "limit exceeds 2000", nil)
		}
		opts.Limit = optional.NewInt32(int32(limit))
	}

	if err := c.waitREST(ctx); err != nil {
		return nil, err
	}
	rows, _, err := c.gate.FuturesApi.ListFuturesCandlesticks(
		c.publicCtx(ctx), c.settleStr(), gateContract, opts)
	if err != nil {
		return nil, mapGateAPIError(err)
	}

	out := make([]*perp.CandlestickSnapshot, 0, len(rows))
	for i := range rows {
		snap := candlestickFromREST(gateContract, interval, &rows[i])
		if snap == nil {
			continue
		}
		out = append(out, snap)
	}
	return out, nil
}

func candlestickFromREST(gateContract, interval string, row *gateapi.FuturesCandlestick) *perp.CandlestickSnapshot {
	if row == nil {
		return nil
	}
	ts := int64(row.T)
	if ts <= 0 {
		return nil
	}
	return &perp.CandlestickSnapshot{
		Contract:     perp.CanonicalContract(gateContract),
		Interval:     interval,
		Open:         strings.TrimSpace(row.O),
		High:         strings.TrimSpace(row.H),
		Low:          strings.TrimSpace(row.L),
		Close:        strings.TrimSpace(row.C),
		Volume:       strings.TrimSpace(row.V),
		VolumeBase:   strings.TrimSpace(row.Sum),
		TimestampSec: ts,
		WindowClosed: true,
	}
}
