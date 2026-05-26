package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Historical account data from persistence (not live balance/position)",
	}

	cmd.AddCommand(newAccountSnapshotsCmd())

	return cmd
}

func newAccountSnapshotsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "snapshots",
		Short:   "Persisted account snapshots from SQLite (equity curve / history)",
		Long:    "Reads historical snapshots written by the engine. For live balance use `signalix balance`; for live positions use `signalix position list`.",
		Aliases: []string{"snapshot"},
	}

	cmd.AddCommand(newAccountSnapshotsListCmd())
	cmd.AddCommand(newAccountSnapshotsLatestCmd())

	return cmd
}

func newAccountSnapshotsListCmd() *cobra.Command {
	var (
		since   string
		until   string
		limit   int32
		details bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List account snapshots (ListAccountSnapshots RPC)",
		Long:  "Default table output shows summary columns only. Use -o json/yaml with --details for full balance and positions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			startMs, endMs, err := parseListTimeFlags(since, until)
			if err != nil {
				return err
			}
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListAccountSnapshots(rpc, &enginev1.ListAccountSnapshotsRequest{
					StartAtUnixMs:  startMs,
					EndAtUnixMs:    endMs,
					Limit:          limit,
					IncludeDetails: details,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintAccountSnapshotList(reply)
			})
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Include snapshots at or after this time (RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Include snapshots at or before this time (RFC3339)")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Max rows (0 = server default 5000)")
	cmd.Flags().BoolVar(&details, "details", false, "Include balance and positions in json/yaml output")

	return cmd
}

func newAccountSnapshotsLatestCmd() *cobra.Command {
	var details bool

	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Show latest account snapshot (GetLatestAccountSnapshot RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetLatestAccountSnapshot(rpc, &enginev1.GetLatestAccountSnapshotRequest{
					IncludeDetails: details,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintLatestAccountSnapshot(reply)
			})
		},
	}

	cmd.Flags().BoolVar(&details, "details", false, "Include balance and positions in json/yaml output")

	return cmd
}
