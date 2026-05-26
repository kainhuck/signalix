package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultGRPCAddr    = "127.0.0.1:50051"
	EnvGRPCAddr        = "SIGNALIX_GRPC_ADDR"
	EnvCLIConfig       = "SIGNALIX_CLI_CONFIG"
	DefaultConfigRel   = ".signalix/cli.toml"
	DefaultOutput      = "table"
	DefaultTimeout     = 10 * time.Second
	DefaultTimeoutText = "10s"
)

// FileConfig holds values loaded from cli.toml.
type FileConfig struct {
	Output  string
	Timeout time.Duration
	Token   string
}

// ResolveOptions captures explicit CLI flag values (empty means unset).
type ResolveOptions struct {
	AddrFlag    string
	OutputFlag  string
	TimeoutFlag string
	TokenFlag   string
	ConfigFlag  string
}

// Settings is the merged runtime configuration for a command invocation.
type Settings struct {
	Addr    string
	Output  string
	Timeout time.Duration
	Token   string
}

// DefaultConfigPath returns ~/.signalix/cli.toml.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultConfigRel), nil
}

// ResolveConfigPath picks the cli.toml path: flag → SIGNALIX_CLI_CONFIG → default.
func ResolveConfigPath(flagPath string) (string, error) {
	if strings.TrimSpace(flagPath) != "" {
		return expandHome(flagPath)
	}
	if env := strings.TrimSpace(os.Getenv(EnvCLIConfig)); env != "" {
		return expandHome(env)
	}
	return DefaultConfigPath()
}

func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[2:]), nil
}

// LoadCLIConfig reads cli.toml. A missing file returns zero values without error.
func LoadCLIConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("cli config: %w", err)
	}
	if info.IsDir() {
		return cfg, fmt.Errorf("cli config: %s is a directory", path)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return cfg, fmt.Errorf("cli config: %w", err)
	}

	cfg.Output = strings.TrimSpace(v.GetString("defaults.output"))
	if raw := strings.TrimSpace(v.GetString("defaults.timeout")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return cfg, fmt.Errorf("cli config defaults.timeout: %w", err)
		}
		cfg.Timeout = d
	}
	cfg.Token = v.GetString("auth.token")
	return cfg, nil
}

// ResolveAddr returns gRPC address: flag → SIGNALIX_GRPC_ADDR → default.
func ResolveAddr(flagAddr string) string {
	if strings.TrimSpace(flagAddr) != "" {
		return strings.TrimSpace(flagAddr)
	}
	if env := strings.TrimSpace(os.Getenv(EnvGRPCAddr)); env != "" {
		return env
	}
	return DefaultGRPCAddr
}

// ResolveSettings merges flag, file, and hardcoded defaults (flag > toml > default).
func ResolveSettings(opts ResolveOptions, file FileConfig) (Settings, error) {
	out := strings.TrimSpace(opts.OutputFlag)
	if out == "" {
		out = strings.TrimSpace(file.Output)
	}
	if out == "" {
		out = DefaultOutput
	}
	if out != "table" && out != "json" && out != "yaml" {
		return Settings{}, fmt.Errorf("invalid output format %q (want table, json, or yaml)", out)
	}

	timeout := DefaultTimeout
	if strings.TrimSpace(opts.TimeoutFlag) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(opts.TimeoutFlag))
		if err != nil {
			return Settings{}, fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = d
	} else if file.Timeout > 0 {
		timeout = file.Timeout
	}

	token := opts.TokenFlag
	if token == "" {
		token = file.Token
	}

	return Settings{
		Addr:    ResolveAddr(opts.AddrFlag),
		Output:  out,
		Timeout: timeout,
		Token:   token,
	}, nil
}
