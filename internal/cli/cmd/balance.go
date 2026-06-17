package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newBalanceCmd() *cobra.Command {
	var market string
	var currency string

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show account balance (GetBalance RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			market, err := normalizeMarketFlag(market)
			if err != nil {
				return err
			}
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetBalance(rpc, &enginev1.GetBalanceRequest{
					Market:   market,
					Currency: currency,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintBalance(reply)
			})
		},
	}
	addMarketFlag(cmd, &market)
	cmd.Flags().StringVar(&currency, "currency", "USDT", "Balance currency")
	return cmd
}
