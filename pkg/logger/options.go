package logger

import "io"

// HandlerType 日志实现类型
type HandlerType int

const (
	HandlerTypeZap  HandlerType = iota // zap 实现
	HandlerTypeSlog                    // slog 标准库实现
)

// Config 日志配置
type Config struct {
	// 基础配置
	level     Level
	handler   HandlerType
	encoding  string // "json" | "console"
	addSource bool
	output    io.Writer

	// 文件轮转配置
	filename   string
	maxSize    int // MB
	maxBackups int
	maxAge     int // days
	compress   bool

	// 预设公共字段
	fields []any
}

// Option 配置选项函数
type Option func(*Config)

func defaultConfig() *Config {
	return &Config{
		level:      LevelInfo,
		handler:    HandlerTypeZap,
		encoding:   "json",
		addSource:  true,
		maxSize:    100,
		maxBackups: 5,
		maxAge:     30,
		compress:   true,
	}
}

// WithLevel 设置日志级别
func WithLevel(level Level) Option {
	return func(c *Config) {
		c.level = level
	}
}

// WithHandler 选择日志实现 (HandlerTypeZap | HandlerTypeSlog)
func WithHandler(h HandlerType) Option {
	return func(c *Config) {
		c.handler = h
	}
}

// WithEncoding 设置输出格式 "json" | "console"
func WithEncoding(encoding string) Option {
	return func(c *Config) {
		c.encoding = encoding
	}
}

// WithAddSource 是否记录调用位置
func WithAddSource(addSource bool) Option {
	return func(c *Config) {
		c.addSource = addSource
	}
}

// WithOutput 设置输出目标（优先于文件配置）
func WithOutput(w io.Writer) Option {
	return func(c *Config) {
		c.output = w
	}
}

// WithFile 设置日志文件路径（启用文件轮转）
func WithFile(filename string) Option {
	return func(c *Config) {
		c.filename = filename
	}
}

// WithMaxSize 单个日志文件最大大小 (MB)
func WithMaxSize(size int) Option {
	return func(c *Config) {
		c.maxSize = size
	}
}

// WithMaxBackups 最大备份文件数
func WithMaxBackups(n int) Option {
	return func(c *Config) {
		c.maxBackups = n
	}
}

// WithMaxAge 最大保留天数
func WithMaxAge(days int) Option {
	return func(c *Config) {
		c.maxAge = days
	}
}

// WithCompress 是否压缩归档文件
func WithCompress(compress bool) Option {
	return func(c *Config) {
		c.compress = compress
	}
}

// WithFields 预设公共字段
func WithFields(args ...any) Option {
	return func(c *Config) {
		c.fields = args
	}
}

// ============================================================
// 预设配置
// ============================================================

// Development 开发环境预设
func Development() Option {
	return func(c *Config) {
		c.level = LevelDebug
		c.encoding = "console"
		c.addSource = true
	}
}

// Production 生产环境预设
func Production() Option {
	return func(c *Config) {
		c.level = LevelInfo
		c.encoding = "json"
		c.addSource = true
	}
}
