package logger

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{}

var ctxKey = contextKey{}

// ctxData 存储在 context 中的日志数据
type ctxData struct {
	args   []any // slog 风格的参数
	logger Logger
}

// WithContextFields 向 context 中注入日志字段
// 可多次调用，字段会累积合并
func WithContextFields(ctx context.Context, args ...any) context.Context {
	existing := contextArgs(ctx)
	merged := make([]any, 0, len(existing)+len(args))
	merged = append(merged, existing...)
	merged = append(merged, args...)
	return context.WithValue(ctx, ctxKey, &ctxData{args: merged})
}

// WithLogger 向 context 中注入完整的 Logger 实例
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey, &ctxData{logger: l})
}

// FromContext 从 context 中获取 Logger
//
//	优先返回注入的 Logger 实例
//	其次返回携带 context 字段的 default Logger
//	最终返回 default Logger
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Default()
	}

	data, ok := ctx.Value(ctxKey).(*ctxData)
	if !ok {
		return Default()
	}

	// 优先返回完整的 Logger 实例
	if data.logger != nil {
		return data.logger
	}

	// 返回携带 context 字段的 Logger
	if len(data.args) > 0 {
		return Default().With(data.args...)
	}

	return Default()
}

// L 是 FromContext 的简写
func L(ctx context.Context) Logger {
	return FromContext(ctx)
}

// ============================================================
// 内部辅助函数
// ============================================================

// contextArgs 提取 context 中的原始参数
func contextArgs(ctx context.Context) []any {
	if ctx == nil {
		return nil
	}
	data, ok := ctx.Value(ctxKey).(*ctxData)
	if !ok || data == nil {
		return nil
	}
	return data.args
}

// contextZapFields 提取 context 中的参数并转换为 zap.Field
func contextZapFields(ctx context.Context) []zap.Field {
	args := contextArgs(ctx)
	if len(args) == 0 {
		return nil
	}
	return argsToZapFields(args)
}
