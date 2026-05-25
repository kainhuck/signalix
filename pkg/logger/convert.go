package logger

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ============================================================
// slog args → zap fields 转换
//
// 支持三种传参风格:
//   1. slog.Attr:       logger.Info("msg", slog.String("k","v"))
//   2. key-value 对:    logger.Info("msg", "key", "value")
//   3. zap.Field:       logger.Info("msg", zap.String("k","v"))
// ============================================================

func argsToZapFields(args []any) []zap.Field {
	if len(args) == 0 {
		return nil
	}

	fields := make([]zap.Field, 0, len(args)/2+1)

	for i := 0; i < len(args); i++ {
		switch v := args[i].(type) {
		case slog.Attr:
			fields = append(fields, attrToZapField(v))

		case zap.Field:
			fields = append(fields, v)

		case string:
			// slog 风格: "key", value, "key", value
			if i+1 < len(args) {
				fields = append(fields, zap.Any(v, args[i+1]))
				i++ // 跳过 value
			} else {
				fields = append(fields, zap.String("!BADKEY", v))
			}

		default:
			fields = append(fields, zap.Any("!BADKEY", v))
		}
	}

	return fields
}

// attrToZapField 将 slog.Attr 转换为 zap.Field
func attrToZapField(attr slog.Attr) zap.Field {
	if attr.Equal(slog.Attr{}) {
		return zap.Skip()
	}

	// 先 Resolve LogValuer
	attr.Value = attr.Value.Resolve()

	switch attr.Value.Kind() {
	case slog.KindString:
		return zap.String(attr.Key, attr.Value.String())
	case slog.KindInt64:
		return zap.Int64(attr.Key, attr.Value.Int64())
	case slog.KindUint64:
		return zap.Uint64(attr.Key, attr.Value.Uint64())
	case slog.KindFloat64:
		return zap.Float64(attr.Key, attr.Value.Float64())
	case slog.KindBool:
		return zap.Bool(attr.Key, attr.Value.Bool())
	case slog.KindDuration:
		return zap.Duration(attr.Key, attr.Value.Duration())
	case slog.KindTime:
		return zap.Time(attr.Key, attr.Value.Time())
	case slog.KindGroup:
		attrs := attr.Value.Group()
		if len(attrs) == 0 {
			return zap.Skip()
		}
		if attr.Key == "" {
			// 内联组：展开为多个字段，取第一个（zap 限制）
			if len(attrs) > 0 {
				return attrToZapField(attrs[0])
			}
			return zap.Skip()
		}
		return zap.Any(attr.Key, groupToMap(attrs))
	default:
		return zap.Any(attr.Key, attr.Value.Any())
	}
}

// groupToMap 将 slog Group 转换为 map（用于 zap.Any）
func groupToMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		a.Value = a.Value.Resolve()
		if a.Value.Kind() == slog.KindGroup {
			m[a.Key] = groupToMap(a.Value.Group())
		} else {
			m[a.Key] = a.Value.Any()
		}
	}
	return m
}

// ============================================================
// Level 转换
// ============================================================

func toZapLevel(level Level) zapcore.Level {
	switch {
	case level < LevelInfo:
		return zapcore.DebugLevel
	case level < LevelWarn:
		return zapcore.InfoLevel
	case level < LevelError:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}
