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
	var market string
	cmd := &cobra.Command{
		Use:   "ticker [symbol]",
		Short: "Show cached ticker for a symbol (GetTicker RPC)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			market, err := normalizeMarketFlag(market)
			if err != nil {
				return err
			}
			symbol := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetTicker(rpc, &enginev1.GetTickerRequest{Symbol: symbol, Market: market})
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
	addMarketFlag(cmd, &market)
	return cmd
}

func newMarketKlinesCmd() *cobra.Command {
	var interval string
	var limit int32
	var market string

	cmd := &cobra.Command{
		Use:   "klines [symbol]",
		Short: "List closed klines from engine buffer (GetKlines RPC)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			market, err := normalizeMarketFlag(market)
			if err != nil {
				return err
			}
			symbol := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetKlines(rpc, &enginev1.GetKlinesRequest{
					Symbol:   symbol,
					Interval: interval,
					Limit:    limit,
					Market:   market,
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
	addMarketFlag(cmd, &market)
	_ = cmd.MarkFlagRequired("interval")

	return cmd
}

func newMarketTickersCmd() *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:   "tickers",
		Short: "List all cached tickers (ListTickers RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			market, err := normalizeMarketFlag(market)
			if err != nil {
				return err
			}
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListTickers(rpc, &enginev1.ListTickersRequest{Market: market})
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
	addMarketFlag(cmd, &market)
	return cmd
}
