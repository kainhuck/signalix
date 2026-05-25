package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	enginev1 "github.com/kainhuck/signalix/api/gen/go/signalix/engine/v1"
	"github.com/kainhuck/signalix/internal/app/engine"
	"github.com/kainhuck/signalix/pkg/logger"
	"google.golang.org/grpc"
)

// Options gRPC 监听与可选鉴权。
type Options struct {
	Addr            string // 例如 127.0.0.1:50051
	Token           string // 非空则要求 metadata authorization: Bearer <token>
	InsecureBindAll bool   // 对应 config grpc.insecure_bind_all
}

// NewServer 创建已注册 Engine 服务的 gRPC Server（未开始 Serve）。
func NewServer(eng *engine.Engine, opts Options) (*grpc.Server, net.Listener, error) {
	if err := VerifyLoopbackListenAddr(opts.Addr, opts.InsecureBindAll); err != nil {
		return nil, nil, err
	}
	lis, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("grpc listen: %w", err)
	}
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			requestIDUnaryServerInterceptor(),
			authUnaryServerInterceptor(opts.Token),
		),
		grpc.ChainStreamInterceptor(
			requestIDStreamServerInterceptor(),
			authStreamServerInterceptor(opts.Token),
		),
	)
	enginev1.RegisterEngineServer(s, NewEngineService(eng))
	return s, lis, nil
}

// ServeBlocking 在调用 goroutine 中阻塞于 gRPC Serve；ctx 取消时 GracefulStop。
// 典型用法：与 signal.NotifyContext 组合作为进程主阻塞点。
func ServeBlocking(ctx context.Context, eng *engine.Engine, opts Options) error {
	srv, lis, err := NewServer(eng, opts)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		st, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-st.Done():
			srv.Stop()
		}
		<-errCh
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}
}

// ServeBackground 在独立 goroutine 中 Serve，返回用于关停的函数（测试或嵌入其它编排时使用）。
func ServeBackground(eng *engine.Engine, opts Options) (shutdown func(), err error) {
	srv, lis, err := NewServer(eng, opts)
	if err != nil {
		return nil, err
	}
	go func() {
		if serveErr := srv.Serve(lis); serveErr != nil {
			logger.Error("grpc server exited", logger.Any("error", serveErr))
		}
	}()
	return func() {
		st, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-st.Done():
			srv.Stop()
		}
		_ = lis.Close()
	}, nil
}
