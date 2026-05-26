package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli"
	"github.com/kainhuck/signalix/internal/cli/output"
)

func watchOrderEvents(ctx context.Context, client enginev1.EngineClient, pr *output.Printer, header bool) error {
	stream, err := client.SubscribeOrderEvents(ctx, &enginev1.SubscribeOrderEventsRequest{})
	if err != nil {
		return formatRPCError(err)
	}

	if header && !pr.UsesOrderWatchNDJSON() {
		if err := pr.PrintOrderEventHeader(); err != nil {
			return err
		}
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		order, err := stream.Recv()
		if err != nil {
			if mapped := cli.FormatStreamError(runtime.settings.Addr, ctx, err); mapped != nil {
				return mapped
			}
			return nil
		}
		if err := pr.PrintOrderEvent(order); err != nil {
			return err
		}
	}
}
