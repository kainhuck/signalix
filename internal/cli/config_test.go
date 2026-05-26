package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/cli"
)

func TestLoadCLIConfig_missingFile(t *testing.T) {
	cfg, err := cli.LoadCLIConfig(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != "" || cfg.Timeout != 0 || cfg.Token != "" {
		t.Fatalf("expected zero config, got %+v", cfg)
	}
}

func TestLoadCLIConfig_defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.toml")
	content := `[defaults]
output = "json"
timeout = "5s"

[auth]
token = "secret"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := cli.LoadCLIConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != "json" {
		t.Fatalf("output: got %q", cfg.Output)
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("timeout: got %v", cfg.Timeout)
	}
	if cfg.Token != "secret" {
		t.Fatalf("token: got %q", cfg.Token)
	}
}

func TestResolveAddr_flagOverridesEnv(t *testing.T) {
	t.Setenv(cli.EnvGRPCAddr, "env:50051")
	if got := cli.ResolveAddr("flag:50051"); got != "flag:50051" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveAddr_envFallback(t *testing.T) {
	t.Setenv(cli.EnvGRPCAddr, "env:50051")
	if got := cli.ResolveAddr(""); got != "env:50051" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveAddr_default(t *testing.T) {
	t.Setenv(cli.EnvGRPCAddr, "")
	if got := cli.ResolveAddr(""); got != cli.DefaultGRPCAddr {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSettings_priority(t *testing.T) {
	file := cli.FileConfig{
		Output:  "yaml",
		Timeout: 7 * time.Second,
		Token:   "file-token",
	}
	settings, err := cli.ResolveSettings(cli.ResolveOptions{
		OutputFlag:  "json",
		TimeoutFlag: "3s",
		TokenFlag:   "flag-token",
	}, file)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Output != "json" {
		t.Fatalf("output: got %q", settings.Output)
	}
	if settings.Timeout != 3*time.Second {
		t.Fatalf("timeout: got %v", settings.Timeout)
	}
	if settings.Token != "flag-token" {
		t.Fatalf("token: got %q", settings.Token)
	}

	settings, err = cli.ResolveSettings(cli.ResolveOptions{}, file)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Output != "yaml" || settings.Timeout != 7*time.Second || settings.Token != "file-token" {
		t.Fatalf("file defaults not applied: %+v", settings)
	}
}

func TestResolveSettings_invalidOutput(t *testing.T) {
	_, err := cli.ResolveSettings(cli.ResolveOptions{OutputFlag: "xml"}, cli.FileConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveConfigPath_env(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.toml")
	t.Setenv(cli.EnvCLIConfig, path)
	got, err := cli.ResolveConfigPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q", got)
	}
}
