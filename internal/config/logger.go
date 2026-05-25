package config

import (
	"fmt"
	"strings"

	"github.com/kainhuck/signalix/pkg/logger"
)

// LoggerOptions 根据 [log] 段生成 logger.Init 选项。
func (l LogConfig) LoggerOptions() ([]logger.Option, error) {
	level, err := parseLogLevel(l.LogLevel)
	if err != nil {
		return nil, err
	}
	opts := []logger.Option{
		logger.WithHandler(logger.HandlerTypeZap),
		logger.WithLevel(level),
		logger.WithAddSource(true),
	}
	if f := strings.TrimSpace(l.File); f != "" {
		opts = append(opts,
			logger.WithEncoding("json"),
			logger.WithFile(f),
			logger.WithMaxSize(positiveOr(l.MaxSizeMB, 100)),
			logger.WithMaxBackups(positiveOr(l.MaxBackups, 5)),
			logger.WithMaxAge(positiveOr(l.MaxAgeDays, 30)),
			logger.WithCompress(l.Compress),
		)
	} else {
		opts = append(opts, logger.WithEncoding("console"))
	}
	return opts, nil
}

func parseLogLevel(s string) (logger.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return logger.LevelDebug, nil
	case "info", "":
		return logger.LevelInfo, nil
	case "warn", "warning":
		return logger.LevelWarn, nil
	case "error":
		return logger.LevelError, nil
	default:
		return logger.LevelInfo, fmt.Errorf("unknown log.log_level %q", s)
	}
}

func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
