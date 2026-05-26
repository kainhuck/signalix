package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newKillSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill-switch",
		Short: "Emergency kill switch control",
	}

	cmd.AddCommand(newKillSwitchStatusCmd())
	cmd.AddCommand(newKillSwitchActivateCmd())
	cmd.AddCommand(newKillSwitchDeactivateCmd())

	return cmd
}

func newKillSwitchStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show kill switch status (GetKillSwitchStatus RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetKillSwitchStatus(rpc, &enginev1.GetKillSwitchStatusRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintKillSwitchStatus(reply)
			})
		},
	}
}

func newKillSwitchActivateCmd() *cobra.Command {
	var reason string
	var cancelOpenOrders bool

	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Activate kill switch (reject new opens; ActivateKillSwitch RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ActivateKillSwitch(rpc, &enginev1.ActivateKillSwitchRequest{
					Reason:           reason,
					CancelOpenOrders: cancelOpenOrders,
				})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintActivateKillSwitch(reply)
			})
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason recorded with activation")
	cmd.Flags().BoolVar(&cancelOpenOrders, "cancel-open-orders", false, "Cancel all open orders when activating")

	return cmd
}

func newKillSwitchDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate kill switch (DeactivateKillSwitch RPC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.DeactivateKillSwitch(rpc, &enginev1.DeactivateKillSwitchRequest{})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintDeactivateKillSwitch(reply)
			})
		},
	}
}
