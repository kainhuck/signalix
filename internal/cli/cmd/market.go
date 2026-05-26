package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newMarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Read-only market data from engine cache",
	}

	cmd.AddCommand(newMarketTickerCmd())
	cmd.AddCommand(newMarketKlinesCmd())
	cmd.AddCommand(newMarketTickersCmd())

	return cmd
}

func newMarketTickerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ticker [symbol]",
		Short: "Show cached ticker for a symbol (GetTicker RPC)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetTicker(rpc, &enginev1.GetTickerRequest{Symbol: symbol})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintTicker(reply)
			})
		},
	}
}

func newMarketKlinesCmd() *cobra.Command {
	var interval string
	var limit int32

	cmd := &cobra.Command{
		Use:   "klines [symbol]",
		Short: "List closed klines from engine buffer (GetKlines RPC)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetKlines(rpc, &enginev1.GetKlinesRequest{
					Symbol:   symbol,
					Interval: interval,
					Limit:    limit,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintKlines(reply)
			})
		},
	}

	cmd.Flags().StringVarP(&interval, "interval", "i", "", "Kline interval (required, e.g. 5m, 1h)")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Max klines (0 = server default 100)")
	_ = cmd.MarkFlagRequired("interval")

	return cmd
}

func newMarketTickersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tickers",
		Short: "List all cached tickers (ListTickers RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListTickers(rpc, &enginev1.ListTickersRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintTickerList(reply)
			})
		},
	}
}
