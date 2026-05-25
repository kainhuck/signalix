package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// 编译期接口检查
var _ internalLogger = (*slogLogger)(nil)

type slogLogger struct {
	sl      *slog.Logger
	handler slog.Handler
}

func newSlogLogger(cfg *Config) (*slogLogger, error) {
	// 输出目标
	writers := make([]io.Writer, 0, 2)
	if cfg.output != nil {
		writers = append(writers, cfg.output)
	} else {
		writers = append(writers, os.Stdout)
	}

	if cfg.filename != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   cfg.filename,
			MaxSize:    cfg.maxSize,
			MaxBackups: cfg.maxBackups,
			MaxAge:     cfg.maxAge,
			Compress:   cfg.compress,
		})
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = io.MultiWriter(writers...)
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.level,
		AddSource: cfg.addSource,
	}

	var handler slog.Handler
	switch cfg.encoding {
	case "console":
		handler = slog.NewTextHandler(w, opts)
	default:
		handler = slog.NewJSONHandler(w, opts)
	}

	sl := slog.New(handler)

	l := &slogLogger{
		sl:      sl,
		handler: handler,
	}

	if len(cfg.fields) > 0 {
		newSl := sl.With(cfg.fields...)
		return &slogLogger{
			sl:      newSl,
			handler: newSl.Handler(),
		}, nil
	}

	return l, nil
}

// ============================================================
// 核心日志方法：通过 runtime.Callers 手动控制 source 信息
// ============================================================

func (l *slogLogger) logWithCallerSkip(ctx context.Context, skip int, level Level, msg string, args ...any) {
	if !l.sl.Enabled(ctx, level) {
		return
	}

	// 手动获取调用方 PC
	var pcs [1]uintptr
	runtime.Callers(skip+1, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)

	// 从 context 提取附加字段
	if ctxArgs := contextArgs(ctx); len(ctxArgs) > 0 {
		r.Add(ctxArgs...)
	}

	_ = l.handler.Handle(ctx, r)
}

// ============================================================
// 实现公开 Logger 接口 (skip=2，与 zapLogger 一致：跳过本类型 Info + logWithCallerSkip)
// ============================================================

func (l *slogLogger) Debug(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelDebug, msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelInfo, msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelWarn, msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelError, msg, args...)
}

func (l *slogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelDebug, msg, args...)
}

func (l *slogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelInfo, msg, args...)
}

func (l *slogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelWarn, msg, args...)
}

func (l *slogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelError, msg, args...)
}

// ============================================================
// With / WithGroup / 其他
// ============================================================

func (l *slogLogger) With(args ...any) Logger {
	newSl := l.sl.With(args...)
	return &slogLogger{
		sl:      newSl,
		handler: newSl.Handler(),
	}
}

func (l *slogLogger) WithGroup(name string) Logger {
	newSl := l.sl.WithGroup(name)
	return &slogLogger{
		sl:      newSl,
		handler: newSl.Handler(),
	}
}

func (l *slogLogger) Enabled(ctx context.Context, level Level) bool {
	return l.sl.Enabled(ctx, level)
}

func (l *slogLogger) Handler() slog.Handler {
	return l.handler
}

func (l *slogLogger) sync() error {
	// slog 标准库无需 sync
	return nil
}
