package cli_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kainhuck/signalix/internal/cli"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFormatGRPCError_unauthenticated(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", time.Second, status.Error(codes.Unauthenticated, "bad"))
	if err == nil || err.Error() != "authentication failed: check --token or cli.toml [auth].token" {
		t.Fatalf("got %v", err)
	}
}

func TestFormatGRPCError_failedPreconditionEngine(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", time.Second, status.Error(codes.FailedPrecondition, "engine not running"))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg != "引擎未运行，请先启动 signalixd: engine not running" {
		t.Fatalf("got %q", msg)
	}
}

func TestFormatGRPCError_deadline(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", 2*time.Second, status.Error(codes.DeadlineExceeded, "timeout"))
	if err == nil || err.Error() != "request timed out after 2s" {
		t.Fatalf("got %v", err)
	}
}

func TestFormatGRPCError_unavailable(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:9", time.Second, status.Error(codes.Unavailable, "connection refused"))
	if err == nil {
		t.Fatal("expected error")
	}
	want := "cannot connect to 127.0.0.1:9: connection refused"
	if err.Error() != want {
		t.Fatalf("got %q", err.Error())
	}
}

func TestFormatGRPCError_strategyNotFound(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", time.Second, status.Error(codes.NotFound, `strategy "foo" not in catalog`))
	if err == nil || err.Error() != `strategy "foo" not in catalog` {
		t.Fatalf("got %v", err)
	}
}

func TestFormatGRPCError_startInternal(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", time.Second, status.Error(codes.Internal, "start strategy: boom"))
	if err == nil || err.Error() != "failed to start strategy: boom" {
		t.Fatalf("got %v", err)
	}
}

func TestFormatGRPCError_other(t *testing.T) {
	err := cli.FormatGRPCError("127.0.0.1:50051", time.Second, status.Error(codes.Internal, "boom"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, err) {
		t.Fatalf("got %v", err)
	}
}
