package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newStrategyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "strategy",
		Short: "Strategy catalog and lifecycle",
	}

	cmd.AddCommand(newStrategyListCmd())
	cmd.AddCommand(newStrategyStatusCmd())
	cmd.AddCommand(newStrategyStartCmd())
	cmd.AddCommand(newStrategyStopCmd())
	cmd.AddCommand(newStrategyReloadCmd())

	return cmd
}

func newStrategyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List strategies in catalog with runtime fields",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListStrategies(rpc, &enginev1.ListStrategiesRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintStrategyList(reply)
			})
		},
	}
}

func newStrategyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show runtime status for one strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetStrategyStatus(rpc, &enginev1.GetStrategyStatusRequest{Name: name})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintStrategyStatus(reply)
			})
		},
	}
}

func newStrategyStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [name]",
		Short: "Start a strategy process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				_, err := client.StartStrategy(rpc, &enginev1.StartStrategyRequest{Name: name})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintStrategyAction(name, "started")
			})
		},
	}
}

func newStrategyStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running strategy process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				_, err := client.StopStrategy(rpc, &enginev1.StopStrategyRequest{Name: name})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintStrategyAction(name, "stopped")
			})
		},
	}
}

func newStrategyReloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Rescan strategies directory and refresh catalog (does not auto start/stop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ReloadStrategies(rpc, &enginev1.ReloadStrategiesRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintReloadResult(reply.GetCatalogCount())
			})
		},
	}
}
