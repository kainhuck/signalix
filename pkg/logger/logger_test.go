package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// ============================================================
// 辅助函数
// ============================================================

func initTestLogger(t *testing.T, buf *bytes.Buffer, handlerType HandlerType) {
	t.Helper()
	err := Init(
		WithHandler(handlerType),
		WithLevel(LevelDebug),
		WithEncoding("json"),
		WithOutput(buf),
		WithAddSource(true),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func parseLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
		t.Fatalf("invalid JSON log line: %s\nerror: %v", line, err)
	}
	return m
}

// ============================================================
// 基础功能测试
// ============================================================

func TestZapBasicLog(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	Info("hello world", String("key", "value"), Int("count", 42))

	m := parseLogLine(t, buf.String())

	if m["msg"] != "hello world" {
		t.Errorf("expected msg 'hello world', got %v", m["msg"])
	}
	if m["key"] != "value" {
		t.Errorf("expected key 'value', got %v", m["key"])
	}
	if m["level"] != "info" {
		t.Errorf("expected level 'info', got %v", m["level"])
	}
}

func TestSlogBasicLog(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeSlog)

	Info("slog hello", String("framework", "slog"))

	output := buf.String()
	if !strings.Contains(output, "slog hello") {
		t.Errorf("expected 'slog hello' in output, got: %s", output)
	}
	if !strings.Contains(output, "framework") {
		t.Errorf("expected 'framework' in output, got: %s", output)
	}
}

// ============================================================
// Caller 位置测试（关键！）
// ============================================================

func TestZapCallerGlobalFunction(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	Info("caller test from global") // 这一行的行号应该出现在日志中

	m := parseLogLine(t, buf.String())
	source, ok := m["source"].(string)
	if !ok {
		t.Fatal("source field not found")
	}

	// 应该指向 logger_test.go，而不是 zap_handler.go 或 logger.go
	if !strings.Contains(source, "logger_test.go") {
		t.Errorf("caller should be logger_test.go, got: %s", source)
	}
}

func TestZapCallerInstanceMethod(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	l := Default()
	l.Info("caller test from instance") // 这一行

	m := parseLogLine(t, buf.String())
	source, ok := m["source"].(string)
	if !ok {
		t.Fatal("source field not found")
	}

	if !strings.Contains(source, "logger_test.go") {
		t.Errorf("caller should be logger_test.go, got: %s", source)
	}
}

func TestZapCallerWithLogger(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	child := With(String("module", "auth"))
	child.Info("caller test from child") // 这一行

	m := parseLogLine(t, buf.String())
	source, ok := m["source"].(string)
	if !ok {
		t.Fatal("source field not found")
	}

	if !strings.Contains(source, "logger_test.go") {
		t.Errorf("caller should be logger_test.go, got: %s", source)
	}
}

func TestZapCallerFromContext(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	ctx := WithContextFields(context.Background(),
		String("request_id", "test-123"),
	)
	L(ctx).Info("caller test from context") // 这一行

	m := parseLogLine(t, buf.String())
	source, ok := m["source"].(string)
	if !ok {
		t.Fatal("source field not found")
	}

	if !strings.Contains(source, "logger_test.go") {
		t.Errorf("caller should be logger_test.go, got: %s", source)
	}
}

func TestSlogCallerGlobalFunction(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeSlog)

	Info("slog caller test")

	m := parseLogLine(t, buf.String())
	source, ok := m["source"].(map[string]any)
	if !ok {
		t.Fatal("source field not found")
	}

	file, _ := source["file"].(string)
	if !strings.Contains(file, "logger_test.go") {
		t.Errorf("caller should be logger_test.go, got: %s", file)
	}
}

// ============================================================
// With 子 Logger 测试
// ============================================================

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	child := With(String("service", "auth"), String("version", "v1"))
	child.Info("child message", String("action", "login"))

	m := parseLogLine(t, buf.String())

	if m["service"] != "auth" {
		t.Errorf("expected service 'auth', got %v", m["service"])
	}
	if m["version"] != "v1" {
		t.Errorf("expected version 'v1', got %v", m["version"])
	}
	if m["action"] != "login" {
		t.Errorf("expected action 'login', got %v", m["action"])
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	child := WithGroup("auth")
	child.Info("grouped message")

	m := parseLogLine(t, buf.String())
	if m["logger"] != "auth" {
		t.Errorf("expected logger 'auth', got %v", m["logger"])
	}
}

// ============================================================
// Context 测试
// ============================================================

func TestContextFields(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	ctx := context.Background()
	ctx = WithContextFields(ctx,
		String("request_id", "req-001"),
		String("user_id", "user-123"),
	)

	L(ctx).Info("context message")

	m := parseLogLine(t, buf.String())

	if m["request_id"] != "req-001" {
		t.Errorf("expected request_id 'req-001', got %v", m["request_id"])
	}
	if m["user_id"] != "user-123" {
		t.Errorf("expected user_id 'user-123', got %v", m["user_id"])
	}
}

func TestContextFieldsMerge(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	ctx := context.Background()
	ctx = WithContextFields(ctx, String("request_id", "req-001"))
	ctx = WithContextFields(ctx, String("trace_id", "trace-abc"))

	L(ctx).Info("merged fields")

	m := parseLogLine(t, buf.String())

	if m["request_id"] != "req-001" {
		t.Errorf("expected request_id 'req-001', got %v", m["request_id"])
	}
	if m["trace_id"] != "trace-abc" {
		t.Errorf("expected trace_id 'trace-abc', got %v", m["trace_id"])
	}
}

func TestWithLoggerInContext(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	customLogger := With(String("custom", "true"))
	ctx := WithLogger(context.Background(), customLogger)

	L(ctx).Info("custom logger message")

	m := parseLogLine(t, buf.String())
	if m["custom"] != "true" {
		t.Errorf("expected custom 'true', got %v", m["custom"])
	}
}

// ============================================================
// 日志级别测试
// ============================================================

func TestLogLevel(t *testing.T) {
	var buf bytes.Buffer
	err := Init(
		WithHandler(HandlerTypeZap),
		WithLevel(LevelWarn),
		WithEncoding("json"),
		WithOutput(&buf),
		WithAddSource(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	Debug("should not appear")
	Info("should not appear")
	if buf.Len() > 0 {
		t.Errorf("debug/info logs should be suppressed at warn level")
	}

	Warn("should appear")
	if buf.Len() == 0 {
		t.Error("warn log should appear at warn level")
	}

	buf.Reset()
	Error("error should appear")
	if buf.Len() == 0 {
		t.Error("error log should appear at warn level")
	}
}

func TestEnabled(t *testing.T) {
	var buf bytes.Buffer
	err := Init(
		WithHandler(HandlerTypeZap),
		WithLevel(LevelWarn),
		WithOutput(&buf),
	)
	if err != nil {
		t.Fatal(err)
	}

	l := Default()
	ctx := context.Background()

	if l.Enabled(ctx, LevelDebug) {
		t.Error("debug should not be enabled at warn level")
	}
	if l.Enabled(ctx, LevelInfo) {
		t.Error("info should not be enabled at warn level")
	}
	if !l.Enabled(ctx, LevelWarn) {
		t.Error("warn should be enabled at warn level")
	}
	if !l.Enabled(ctx, LevelError) {
		t.Error("error should be enabled at warn level")
	}
}

// ============================================================
// KV 风格参数测试
// ============================================================

func TestKVStyleArgs(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	// "key", value 松散风格
	Info("kv style", "name", "alice", "age", 25)

	m := parseLogLine(t, buf.String())
	if m["name"] != "alice" {
		t.Errorf("expected name 'alice', got %v", m["name"])
	}
}

func TestMixedArgs(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	// 混合使用 slog.Attr 和 kv 风格
	Info("mixed",
		String("attr_key", "attr_value"),
		"kv_key", "kv_value",
	)

	m := parseLogLine(t, buf.String())
	if m["attr_key"] != "attr_value" {
		t.Errorf("expected attr_key, got %v", m["attr_key"])
	}
	if m["kv_key"] != "kv_value" {
		t.Errorf("expected kv_key, got %v", m["kv_key"])
	}
}

// ============================================================
// 预设字段测试
// ============================================================

func TestWithFieldsOption(t *testing.T) {
	var buf bytes.Buffer
	err := Init(
		WithHandler(HandlerTypeZap),
		WithLevel(LevelDebug),
		WithEncoding("json"),
		WithOutput(&buf),
		WithAddSource(false),
		WithFields(String("app", "myservice"), String("env", "test")),
	)
	if err != nil {
		t.Fatal(err)
	}

	Info("preset fields test")

	m := parseLogLine(t, buf.String())
	if m["app"] != "myservice" {
		t.Errorf("expected app 'myservice', got %v", m["app"])
	}
	if m["env"] != "test" {
		t.Errorf("expected env 'test', got %v", m["env"])
	}
}

// ============================================================
// ContextContext 方法测试
// ============================================================

func TestContextMethods(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	ctx := WithContextFields(context.Background(),
		String("trace", "abc"),
	)

	InfoContext(ctx, "context method test", String("action", "test"))

	m := parseLogLine(t, buf.String())
	if m["trace"] != "abc" {
		t.Errorf("expected trace 'abc', got %v", m["trace"])
	}
	if m["action"] != "test" {
		t.Errorf("expected action 'test', got %v", m["action"])
	}
}

// ============================================================
// Duration / Time 类型测试
// ============================================================

func TestSpecialTypes(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	now := time.Now()
	Info("special types",
		Duration("latency", 250*time.Millisecond),
		Time("timestamp", now),
		Bool("active", true),
		Float64("rate", 0.95),
	)

	m := parseLogLine(t, buf.String())
	if m["active"] != true {
		t.Errorf("expected active true, got %v", m["active"])
	}
	if m["rate"] != 0.95 {
		t.Errorf("expected rate 0.95, got %v", m["rate"])
	}
}

// ============================================================
// Handler 桥接测试
// ============================================================

func TestHandlerBridge(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	handler := Default().Handler()
	if handler == nil {
		t.Fatal("handler should not be nil")
	}

	// 使用 slog.Handler 接口
	ctx := context.Background()
	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("handler should be enabled for info level")
	}
}

// ============================================================
// 并发安全测试
// ============================================================

func TestConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	initTestLogger(t, &buf, HandlerTypeZap)

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			Info("concurrent", Int("goroutine", n))
			With(Int("n", n)).Info("with concurrent")
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkZapGlobalInfo(b *testing.B) {
	var buf bytes.Buffer
	_ = Init(
		WithHandler(HandlerTypeZap),
		WithOutput(&buf),
		WithAddSource(false),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Info("benchmark", String("key", "value"), Int("count", 1))
		}
	})
}

func BenchmarkZapInstanceInfo(b *testing.B) {
	var buf bytes.Buffer
	_ = Init(
		WithHandler(HandlerTypeZap),
		WithOutput(&buf),
		WithAddSource(false),
	)

	l := Default()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Info("benchmark", String("key", "value"), Int("count", 1))
		}
	})
}

func BenchmarkSlogGlobalInfo(b *testing.B) {
	var buf bytes.Buffer
	_ = Init(
		WithHandler(HandlerTypeSlog),
		WithOutput(&buf),
		WithAddSource(false),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Info("benchmark", String("key", "value"), Int("count", 1))
		}
	})
}

func BenchmarkZapWithContext(b *testing.B) {
	var buf bytes.Buffer
	_ = Init(
		WithHandler(HandlerTypeZap),
		WithOutput(&buf),
		WithAddSource(false),
	)

	ctx := WithContextFields(context.Background(),
		String("request_id", "bench-123"),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			L(ctx).Info("benchmark", String("key", "value"))
		}
	})
}
