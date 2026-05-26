package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"
)

func newStrategyCreateCmd() *cobra.Command {
	var (
		templateID      string
		symbols         []string
		interval        string
		historyBars     int32
		subscribeTicker bool
		enabled         bool
		parametersJSON  string
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a strategy from a scaffold template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			req := &enginev1.CreateStrategyRequest{
				Name:       name,
				TemplateId: templateID,
			}
			if len(symbols) > 0 {
				req.Symbols = symbols
			}
			if cmd.Flags().Changed("interval") {
				v := interval
				req.Interval = &v
			}
			if cmd.Flags().Changed("history-bars") {
				v := historyBars
				req.HistoryBars = &v
			}
			if cmd.Flags().Changed("subscribe-ticker") {
				v := subscribeTicker
				req.SubscribeTicker = &v
			}
			if cmd.Flags().Changed("enabled") {
				v := enabled
				req.Enabled = &v
			}
			if strings.TrimSpace(parametersJSON) != "" {
				params, err := parseParametersJSON(parametersJSON)
				if err != nil {
					return err
				}
				req.Parameters = params
			}

			return withClient(cmd.Context(), func(ctx context.Context, client enginev1.EngineClient) error {
				rpc, cancel := rpcCtx(ctx)
				defer cancel()

				reply, err := client.CreateStrategy(rpc, req)
				if err != nil {
					return formatRPCError(err)
				}

				pr, err := newPrinter()
				if err != nil {
					return err
				}
				return pr.PrintCreateStrategy(reply)
			})
		},
	}

	cmd.Flags().StringVar(&templateID, "template", "blank", "Scaffold template ID (trend, blank)")
	cmd.Flags().StringSliceVar(&symbols, "symbol", nil, "Trading symbol (repeatable; replaces template defaults)")
	cmd.Flags().StringVar(&interval, "interval", "", "Override template interval")
	cmd.Flags().Int32Var(&historyBars, "history-bars", 0, "Override template history_bars")
	cmd.Flags().BoolVar(&subscribeTicker, "subscribe-ticker", false, "Override template subscribe_ticker")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Set strategy enabled in config.yaml")
	cmd.Flags().StringVar(&parametersJSON, "parameters-json", "", "JSON object merged into template parameters")

	return cmd
}

func parseParametersJSON(raw string) (*structpb.Struct, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid --parameters-json: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("invalid --parameters-json: expected JSON object")
	}
	return structpb.NewStruct(m)
}
