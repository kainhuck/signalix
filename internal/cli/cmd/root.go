package cmd

import (
	"context"
	"fmt"
	"os"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/cli"
	"github.com/kainhuck/signalix/internal/cli/output"
	"github.com/spf13/cobra"
)

var version = "dev"

var runtime struct {
	addr         string
	output       string
	timeout      string
	token        string
	config       string
	settings     cli.Settings
	configLoaded bool
}

// Execute runs the root command and returns the process exit code.
func Execute() int {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		if ee, ok := err.(*cli.ExitError); ok {
			if ee.Msg != "" {
				fmt.Fprintln(os.Stderr, ee.Msg)
			}
			if ee.Code != 0 {
				return ee.Code
			}
			return 1
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "signalix",
		Short: "Signalix local operations CLI",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return loadRuntime()
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&runtime.addr, "addr", "", "gRPC server address")
	root.PersistentFlags().StringVarP(&runtime.output, "output", "o", "", "Output format: table, json, yaml")
	root.PersistentFlags().StringVar(&runtime.timeout, "timeout", "", "RPC timeout (e.g. 10s)")
	root.PersistentFlags().StringVar(&runtime.token, "token", "", "Bearer token for gRPC auth")
	root.PersistentFlags().StringVar(&runtime.config, "config", "", "CLI config file path")

	root.Version = version
	root.SetVersionTemplate("signalix {{.Version}}\n")

	root.AddCommand(newPingCmd())
	root.AddCommand(newHealthCmd())
	root.AddCommand(newEngineCmd())
	root.AddCommand(newStrategyCmd())
	root.AddCommand(newBalanceCmd())
	root.AddCommand(newPositionCmd())
	root.AddCommand(newAccountCmd())
	root.AddCommand(newOrderCmd())
	root.AddCommand(newMarketCmd())
	root.AddCommand(newKillSwitchCmd())
	root.AddCommand(newCompletionCmd(root))

	return root
}

func loadRuntime() error {
	if runtime.configLoaded {
		return nil
	}
	configPath, err := cli.ResolveConfigPath(runtime.config)
	if err != nil {
		return err
	}
	fileCfg, err := cli.LoadCLIConfig(configPath)
	if err != nil {
		return err
	}
	settings, err := cli.ResolveSettings(cli.ResolveOptions{
		AddrFlag:    runtime.addr,
		OutputFlag:  runtime.output,
		TimeoutFlag: runtime.timeout,
		TokenFlag:   runtime.token,
		ConfigFlag:  runtime.config,
	}, fileCfg)
	if err != nil {
		return err
	}
	runtime.settings = settings
	runtime.configLoaded = true
	return nil
}

func withClient(ctx context.Context, fn func(context.Context, enginev1.EngineClient) error) error {
	client, closeFn, err := cli.DialEngine(runtime.settings.Addr, runtime.settings.Token)
	if err != nil {
		return err
	}
	defer closeFn()
	return fn(ctx, client)
}

func newPrinter() (*output.Printer, error) {
	return output.NewPrinter(runtime.settings.Output, os.Stdout)
}

func rpcCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return cli.RPCContext(ctx, runtime.settings.Timeout)
}

func formatRPCError(err error) error {
	return cli.FormatGRPCError(runtime.settings.Addr, runtime.settings.Timeout, err)
}
