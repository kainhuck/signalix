package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// DialEngine connects to signalixd and returns a client and close function.
func DialEngine(addr, token string) (enginev1.EngineClient, func() error, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if t := strings.TrimSpace(token); t != "" {
		opts = append(opts,
			grpc.WithUnaryInterceptor(authUnaryInterceptor(t)),
			grpc.WithStreamInterceptor(authStreamInterceptor(t)),
		)
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot connect to %s: %w", addr, err)
	}
	return enginev1.NewEngineClient(conn), conn.Close, nil
}

// RPCContext returns a timeout-bound context for a single unary RPC.
func RPCContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

// FormatGRPCError maps gRPC errors to human-readable CLI messages.
func FormatGRPCError(addr string, timeout time.Duration, err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("cannot connect to %s: %v", addr, err)
	}
	msg := st.Message()
	switch st.Code() {
	case codes.Unavailable:
		return fmt.Errorf("cannot connect to %s: %s", addr, msg)
	case codes.Unauthenticated:
		return fmt.Errorf("authentication failed: check --token or cli.toml [auth].token")
	case codes.FailedPrecondition:
		if strings.Contains(strings.ToLower(msg), "engine not running") {
			return fmt.Errorf("引擎未运行，请先启动 signalixd: %s", msg)
		}
		if strings.Contains(strings.ToLower(msg), "projection not ready") {
			return fmt.Errorf("account projection not ready: wait for `signalix health` account_projection=PASS (%s)", msg)
		}
		return fmt.Errorf("%s", msg)
	case codes.NotFound:
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "catalog") || strings.Contains(lower, "strategy") {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("%s", msg)
	case codes.InvalidArgument:
		return fmt.Errorf("%s", msg)
	case codes.Internal:
		if strings.Contains(strings.ToLower(msg), "start strategy") {
			return fmt.Errorf("failed to start strategy: %s", strings.TrimPrefix(msg, "start strategy: "))
		}
		return fmt.Errorf("rpc error: %s %s", st.Code(), msg)
	case codes.DeadlineExceeded:
		return fmt.Errorf("request timed out after %s", timeout)
	default:
		return fmt.Errorf("rpc error: %s %s", st.Code(), msg)
	}
}

// FormatStreamError maps streaming errors; context cancellation returns nil.
func FormatStreamError(addr string, ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return FormatGRPCError(addr, 0, err)
}

func authUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withAuth(ctx, token), method, req, reply, cc, opts...)
	}
}

func authStreamInterceptor(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withAuth(ctx, token), desc, cc, method, opts...)
	}
}

func withAuth(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+strings.TrimSpace(token))
}
