package logger

import (
	"context"
	"log/slog"
	"sync"
)

// Level 直接复用 slog.Level，迁移零成本
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Attr 直接复用 slog.Attr
type Attr = slog.Attr

// 复用 slog 的字段构造函数
var (
	String   = slog.String
	Int      = slog.Int
	Int64    = slog.Int64
	Uint64   = slog.Uint64
	Float64  = slog.Float64
	Bool     = slog.Bool
	Any      = slog.Any
	Duration = slog.Duration
	Time     = slog.Time
	Group    = slog.Group
)

// Logger 公开接口 — 与 slog.Logger 方法签名一致
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)

	With(args ...any) Logger
	WithGroup(name string) Logger
	Enabled(ctx context.Context, level Level) bool
	Handler() slog.Handler
}

// internalLogger 内部接口，支持动态 callerSkip
type internalLogger interface {
	Logger
	logWithCallerSkip(ctx context.Context, skip int, level Level, msg string, args ...any)
	sync() error
}

// ============================================================
// 全局 Logger
// ============================================================

var (
	defaultLogger internalLogger
	once          sync.Once
	mu            sync.RWMutex
)

// Init 初始化全局 Logger
func Init(opts ...Option) error {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var l internalLogger
	var err error

	switch cfg.handler {
	case HandlerTypeSlog:
		l, err = newSlogLogger(cfg)
	default:
		l, err = newZapLogger(cfg)
	}
	if err != nil {
		return err
	}

	mu.Lock()
	defaultLogger = l
	mu.Unlock()

	return nil
}

// getDefault 获取内部 logger（懒初始化）
func getDefault() internalLogger {
	mu.RLock()
	l := defaultLogger
	mu.RUnlock()

	if l == nil {
		once.Do(func() {
			_ = Init()
		})
		mu.RLock()
		l = defaultLogger
		mu.RUnlock()
	}
	return l
}

// Default 获取全局 Logger
func Default() Logger {
	return getDefault()
}

// SetDefault 替换全局 Logger
func SetDefault(l Logger) {
	mu.Lock()
	defer mu.Unlock()

	if il, ok := l.(internalLogger); ok {
		defaultLogger = il
	}
}

// Sync 刷新日志缓冲区（程序退出前调用）
func Sync() error {
	return getDefault().sync()
}

// ============================================================
// 全局便捷函数
//
// 调用链:
//   业务代码 main.go:25       ← 期望显示这里
//     → logger.Info()         ← 全局函数 (skip=2)
//       → logWithCallerSkip()
//         → zap.Info()
// ============================================================

func Debug(msg string, args ...any) {
	getDefault().logWithCallerSkip(context.Background(), 2, LevelDebug, msg, args...)
}

func Info(msg string, args ...any) {
	getDefault().logWithCallerSkip(context.Background(), 2, LevelInfo, msg, args...)
}

func Warn(msg string, args ...any) {
	getDefault().logWithCallerSkip(context.Background(), 2, LevelWarn, msg, args...)
}

func Error(msg string, args ...any) {
	getDefault().logWithCallerSkip(context.Background(), 2, LevelError, msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	getDefault().logWithCallerSkip(ctx, 2, LevelDebug, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	getDefault().logWithCallerSkip(ctx, 2, LevelInfo, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	getDefault().logWithCallerSkip(ctx, 2, LevelWarn, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	getDefault().logWithCallerSkip(ctx, 2, LevelError, msg, args...)
}

func With(args ...any) Logger {
	return Default().With(args...)
}

func WithGroup(name string) Logger {
	return Default().WithGroup(name)
}
