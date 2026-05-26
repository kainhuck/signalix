package cmd

import (
	"context"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
)

func newOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Order queries and control",
	}

	cmd.AddCommand(newOrderGetCmd())
	cmd.AddCommand(newOrderListCmd())
	cmd.AddCommand(newOrderCancelCmd())
	cmd.AddCommand(newOrderWatchCmd())

	return cmd
}

func newOrderGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [order-id]",
		Short: "Show one order by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			orderID := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.GetOrder(rpc, &enginev1.GetOrderRequest{OrderId: orderID})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintOrder(reply)
			})
		},
	}
}

func newOrderListCmd() *cobra.Command {
	var limit int32

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.ListOpenOrders(rpc, &enginev1.ListOpenOrdersRequest{Limit: limit})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintOrderList(reply)
			})
		},
	}

	cmd.Flags().Int32Var(&limit, "limit", 0, "Max orders to return (0 = server default 500)")

	return cmd
}

func newOrderCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [order-id]",
		Short: "Cancel an open order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			orderID := args[0]
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				_, err := client.CancelOrder(rpc, &enginev1.CancelOrderRequest{OrderId: orderID})
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintOrderCancel(orderID)
			})
		},
	}
}

func newOrderWatchCmd() *cobra.Command {
	var noHeader bool

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream order events (SubscribeOrderEvents RPC; yaml uses NDJSON)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return watchOrderEvents(cmd.Context(), client, pr, !noHeader)
			})
		},
	}

	cmd.Flags().BoolVar(&noHeader, "no-header", false, "Omit column header row in table output")

	return cmd
}
