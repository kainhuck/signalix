package grpc

import (
	"errors"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapMarketErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, engine.ErrMarketRouterNotConfigured) {
		return status.Error(codes.FailedPrecondition, "market router not configured")
	}
	if errors.Is(err, engine.ErrTickerNotInCache) {
		return status.Error(codes.NotFound, err.Error())
	}
	msg := err.Error()
	if msg == "symbol is required" || msg == "interval is required" {
		return status.Error(codes.InvalidArgument, msg)
	}
	return status.Errorf(codes.Internal, "market snapshot: %v", err)
}

func tickerToProto(t *perp.TickerSnapshot) *enginev1.Ticker {
	if t == nil {
		return nil
	}
	return &enginev1.Ticker{
		Symbol:          string(t.Contract),
		Last:            t.Last,
		MarkPrice:       t.MarkPrice,
		IndexPrice:      t.IndexPrice,
		FundingRate:     t.FundingRate,
		ChangePct_24H:   t.ChangePct24h,
		Volume_24H:      t.Volume24h,
		Volume_24HBase:  t.Volume24hBase,
		Volume_24HQuote: t.Volume24hQuote,
		OpenInterest:    t.OpenInterest,
		Low_24H:         t.Low24h,
		High_24H:        t.High24h,
		TimestampUnixMs: t.TimestampMillis,
	}
}

func klineToProto(k *models.Kline) *enginev1.Kline {
	if k == nil {
		return nil
	}
	return &enginev1.Kline{
		Symbol:           string(k.Contract),
		Interval:         k.Interval,
		Open:             k.Open,
		High:             k.High,
		Low:              k.Low,
		Close:            k.Close,
		Volume:           k.Volume,
		VolumeBase:       k.VolumeBase,
		TimestampUnixSec: k.TimestampSec,
		WindowClosed:     k.WindowClosed,
	}
}
