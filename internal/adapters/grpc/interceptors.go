package grpc

import (
	"context"
	"strings"

	"github.com/kainhuck/signalix/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const mdRequestID = "x-request-id"

func requestIDUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = withRequestID(ctx)
		return handler(ctx, req)
	}
}

func requestIDStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := withRequestID(ss.Context())
		wrapped := &ctxServerStream{ServerStream: ss, ctx: ctx}
		return handler(srv, wrapped)
	}
}

func withRequestID(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	rid := md.Get(mdRequestID)
	if len(rid) == 0 || rid[0] == "" {
		return ctx
	}
	return logger.WithContextFields(ctx, logger.String("request_id", rid[0]))
}

type ctxServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *ctxServerStream) Context() context.Context { return s.ctx }

func authUnaryServerInterceptor(token string) grpc.UnaryServerInterceptor {
	if strings.TrimSpace(token) == "" {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	t := strings.TrimSpace(token)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := checkBearerToken(ctx, t); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func authStreamServerInterceptor(token string) grpc.StreamServerInterceptor {
	if strings.TrimSpace(token) == "" {
		return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}
	t := strings.TrimSpace(token)
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := checkBearerToken(ss.Context(), t); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func checkBearerToken(ctx context.Context, want string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization")
	}
	const pfx = "Bearer "
	v := strings.TrimSpace(vals[0])
	if !strings.HasPrefix(v, pfx) {
		return status.Error(codes.Unauthenticated, "authorization must be Bearer token")
	}
	if strings.TrimSpace(strings.TrimPrefix(v, pfx)) != want {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return nil
}
