package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli"
	"github.com/kainhuck/signalix/internal/cli/output"
	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	var skipExchangePing bool
	var probe bool

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check engine readiness (GetHealth RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetHealth(rpc, &enginev1.GetHealthRequest{
					SkipExchangePing: skipExchangePing,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				if err := pr.PrintHealth(reply); err != nil {
					return err
				}

				if probe && !output.HealthProbeOK(reply.GetStatus()) {
					return &cli.ExitError{Code: 1}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&skipExchangePing, "skip-exchange-ping", false, "Skip exchange connectivity check")
	cmd.Flags().BoolVar(&probe, "probe", false, "Exit 1 unless status is HEALTHY")

	return cmd
}
