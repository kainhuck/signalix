package logger

import (
	"context"
	"log/slog"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 编译期接口检查
var _ internalLogger = (*zapLogger)(nil)

type zapLogger struct {
	zl     *zap.Logger
	level  *zap.AtomicLevel
	config *Config
}

func newZapLogger(cfg *Config) (*zapLogger, error) {
	atomicLevel := zap.NewAtomicLevelAt(toZapLevel(cfg.level))

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "source",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	switch cfg.encoding {
	case "console":
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 输出目标
	writers := make([]zapcore.WriteSyncer, 0, 2)
	if cfg.output != nil {
		writers = append(writers, zapcore.AddSync(cfg.output))
	} else {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	// 文件轮转
	if cfg.filename != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.filename,
			MaxSize:    cfg.maxSize,
			MaxBackups: cfg.maxBackups,
			MaxAge:     cfg.maxAge,
			Compress:   cfg.compress,
		}
		writers = append(writers, zapcore.AddSync(fileWriter))
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(writers...),
		atomicLevel,
	)

	// 关键：基础 Logger 不设置 CallerSkip
	// CallerSkip 由 logWithCallerSkip 动态设置
	opts := make([]zap.Option, 0, 2)
	if cfg.addSource {
		opts = append(opts, zap.AddCaller())
	}

	zl := zap.New(core, opts...)

	l := &zapLogger{
		zl:     zl,
		level:  &atomicLevel,
		config: cfg,
	}

	// 添加预设字段
	if len(cfg.fields) > 0 {
		return l.withFields(cfg.fields...), nil
	}

	return l, nil
}

// ============================================================
// 实现公开 Logger 接口
//
// 调用链:
//   业务 / exchange client.go:44  ← 期望显示这里
//     → (*zapLogger).Info()       ← 第 1 层包装
//       → logWithCallerSkip()
//         → zl.Info()             ← zap 在此取 caller；需 AddCallerSkip(2) 才能回到业务帧
// ============================================================

func (l *zapLogger) Debug(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelDebug, msg, args...)
}

func (l *zapLogger) Info(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelInfo, msg, args...)
}

func (l *zapLogger) Warn(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelWarn, msg, args...)
}

func (l *zapLogger) Error(msg string, args ...any) {
	l.logWithCallerSkip(context.Background(), 2, LevelError, msg, args...)
}

func (l *zapLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelDebug, msg, args...)
}

func (l *zapLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelInfo, msg, args...)
}

func (l *zapLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelWarn, msg, args...)
}

func (l *zapLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logWithCallerSkip(ctx, 2, LevelError, msg, args...)
}

// ============================================================
// 核心日志方法
// ============================================================

func (l *zapLogger) logWithCallerSkip(ctx context.Context, skip int, level Level, msg string, args ...any) {
	if !l.Enabled(ctx, level) {
		return
	}

	// 合并字段
	fields := argsToZapFields(args)

	// 从 context 提取附加字段
	if ctxFields := contextZapFields(ctx); len(ctxFields) > 0 {
		fields = append(fields, ctxFields...)
	}

	// 动态设置 CallerSkip
	zl := l.zl.WithOptions(zap.AddCallerSkip(skip))

	switch level {
	case LevelDebug:
		zl.Debug(msg, fields...)
	case LevelInfo:
		zl.Info(msg, fields...)
	case LevelWarn:
		zl.Warn(msg, fields...)
	case LevelError:
		zl.Error(msg, fields...)
	default:
		zl.Info(msg, fields...)
	}
}

// ============================================================
// With / WithGroup / 其他
// ============================================================

func (l *zapLogger) withFields(args ...any) *zapLogger {
	fields := argsToZapFields(args)
	return &zapLogger{
		zl:     l.zl.With(fields...),
		level:  l.level,
		config: l.config,
	}
}

func (l *zapLogger) With(args ...any) Logger {
	return l.withFields(args...)
}

func (l *zapLogger) WithGroup(name string) Logger {
	return &zapLogger{
		zl:     l.zl.Named(name),
		level:  l.level,
		config: l.config,
	}
}

func (l *zapLogger) Enabled(_ context.Context, level Level) bool {
	return l.zl.Core().Enabled(toZapLevel(level))
}

func (l *zapLogger) Handler() slog.Handler {
	return &zapSlogBridge{zl: l.zl}
}

func (l *zapLogger) sync() error {
	return l.zl.Sync()
}

// ============================================================
// zap → slog.Handler 桥接
// 用于与需要 slog.Handler 的第三方库集成
// ============================================================

type zapSlogBridge struct {
	zl *zap.Logger
}

func (h *zapSlogBridge) Enabled(_ context.Context, level slog.Level) bool {
	return h.zl.Core().Enabled(toZapLevel(level))
}

func (h *zapSlogBridge) Handle(_ context.Context, record slog.Record) error {
	fields := make([]zap.Field, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		fields = append(fields, attrToZapField(a))
		return true
	})

	switch {
	case record.Level < slog.LevelInfo:
		h.zl.Debug(record.Message, fields...)
	case record.Level < slog.LevelWarn:
		h.zl.Info(record.Message, fields...)
	case record.Level < slog.LevelError:
		h.zl.Warn(record.Message, fields...)
	default:
		h.zl.Error(record.Message, fields...)
	}
	return nil
}

func (h *zapSlogBridge) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make([]zap.Field, 0, len(attrs))
	for _, a := range attrs {
		fields = append(fields, attrToZapField(a))
	}
	return &zapSlogBridge{zl: h.zl.With(fields...)}
}

func (h *zapSlogBridge) WithGroup(name string) slog.Handler {
	return &zapSlogBridge{zl: h.zl.Named(name)}
}
