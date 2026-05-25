package exchange

import "context"

type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type noopLogger struct{}

func (noopLogger) DebugContext(ctx context.Context, msg string, args ...any) {}
func (noopLogger) InfoContext(ctx context.Context, msg string, args ...any)  {}
func (noopLogger) WarnContext(ctx context.Context, msg string, args ...any)  {}
func (noopLogger) ErrorContext(ctx context.Context, msg string, args ...any) {}

// NopLogger 空日志实现，可注入 Client。
var NopLogger Logger = noopLogger{}
