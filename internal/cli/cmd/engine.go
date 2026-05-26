package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newEngineCmd() *cobra.Command {
	engine := &cobra.Command{
		Use:   "engine",
		Short: "Engine metadata commands",
	}

	engine.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show engine version and paths (GetEngineInfo RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetEngineInfo(rpc, &enginev1.GetEngineInfoRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintEngineInfo(reply)
			})
		},
	})

	return engine
}
