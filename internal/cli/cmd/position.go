package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newPositionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "position",
		Short: "Position queries (GetPosition / ListPositions RPC)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "get [symbol]",
		Short: "Show position for one symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetPosition(rpc, &enginev1.GetPositionRequest{Symbol: symbol})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintPosition(reply)
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all open positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListPositions(rpc, &enginev1.ListPositionsRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintPositionList(reply)
			})
		},
	})

	return cmd
}
