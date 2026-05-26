package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newStrategyLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Persisted strategy logs (SQLite via gRPC)",
	}

	cmd.AddCommand(newStrategyLogsListCmd())
	cmd.AddCommand(newStrategyLogsWatchCmd())

	return cmd
}

func newStrategyLogsListCmd() *cobra.Command {
	var (
		strategyName string
		since        string
		until        string
		limit        int32
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List persisted strategy logs (ListStrategyLogs RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			startMs, endMs, err := parseListTimeFlags(since, until)
			if err != nil {
				return err
			}
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListStrategyLogs(rpc, &enginev1.ListStrategyLogsRequest{
					StrategyName:  strategyName,
					StartAtUnixMs: startMs,
					EndAtUnixMs:   endMs,
					Limit:         limit,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintStrategyLogList(reply)
			})
		},
	}

	cmd.Flags().StringVar(&strategyName, "strategy", "", "Filter by strategy name (empty = all)")
	cmd.Flags().StringVar(&since, "since", "", "Include logs at or after this time (RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Include logs at or before this time (RFC3339)")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Max rows (0 = server default 100)")

	return cmd
}

func newStrategyLogsWatchCmd() *cobra.Command {
	var (
		strategyName string
		tail         int32
		noHeader     bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream strategy logs (SubscribeStrategyLogs RPC; yaml uses NDJSON)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return watchStrategyLogs(cmd.Context(), client, &enginev1.SubscribeStrategyLogsRequest{
					StrategyName: strategyName,
					Tail:         tail,
				}, pr, !noHeader)
			})
		},
	}

	cmd.Flags().StringVar(&strategyName, "strategy", "", "Filter by strategy name (empty = all)")
	cmd.Flags().Int32Var(&tail, "tail", 0, "Replay recent rows on connect (0 = server default 50)")
	cmd.Flags().BoolVar(&noHeader, "no-header", false, "Omit column header row in table output")

	return cmd
}
