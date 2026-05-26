package grpc

import (
	"errors"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/internal/app/strategy/scaffold"
	"github.com/kainhuck/signalix/pkg/exchange/perp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapScaffoldErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, scaffold.ErrTemplateNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, scaffold.ErrAlreadyExists) {
		return status.Error(codes.AlreadyExists, err.Error())
	}
	if errors.Is(err, scaffold.ErrInvalidName) || errors.Is(err, scaffold.ErrInvalidConfig) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, engine.ErrStrategyLoaderNotConfigured) {
		return status.Error(codes.FailedPrecondition, "strategy loader not configured")
	}
	msg := err.Error()
	if msg == "template_id is required" {
		return status.Error(codes.InvalidArgument, msg)
	}
	return status.Errorf(codes.Internal, "create strategy: %v", err)
}

func templateMetaToProto(m scaffold.TemplateMeta) *enginev1.StrategyTemplate {
	syms := make([]string, 0, len(m.DefaultSymbols))
	for _, s := range m.DefaultSymbols {
		syms = append(syms, string(s))
	}
	return &enginev1.StrategyTemplate{
		Id:              m.ID,
		Description:     m.Description,
		DefaultSymbols:  syms,
		DefaultInterval: m.DefaultInterval,
	}
}

func createStrategyOptionsFromProto(req *enginev1.CreateStrategyRequest) scaffold.CreateOptions {
	opts := scaffold.CreateOptions{
		Name:       req.GetName(),
		TemplateID: req.GetTemplateId(),
	}
	if syms := req.GetSymbols(); len(syms) > 0 {
		opts.Symbols = make([]perp.Contract, 0, len(syms))
		for _, s := range syms {
			opts.Symbols = append(opts.Symbols, perp.Contract(s))
		}
	}
	if req.Interval != nil {
		v := req.GetInterval()
		opts.Interval = &v
	}
	if req.HistoryBars != nil {
		v := int(req.GetHistoryBars())
		opts.HistoryBars = &v
	}
	if req.SubscribeTicker != nil {
		v := req.GetSubscribeTicker()
		opts.SubscribeTicker = &v
	}
	if req.Enabled != nil {
		v := req.GetEnabled()
		opts.Enabled = &v
	}
	if p := req.GetParameters(); p != nil {
		if m := p.AsMap(); len(m) > 0 {
			opts.Parameters = m
		}
	}
	return opts
}
