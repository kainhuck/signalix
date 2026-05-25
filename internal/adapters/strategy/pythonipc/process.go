package pythonipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/kainhuck/signalix/internal/app/strategy"
	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/pkg/logger"
)

// StrategyProcess Python 子进程适配（实现 strategy.StrategyRuntime）。
type StrategyProcess struct {
	name      string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	heartbeat time.Time
	crashes   []time.Time
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

var _ strategy.StrategyRuntime = (*StrategyProcess)(nil)

// Name 策略名（实现 strategy.StrategyRuntime）。
func (sp *StrategyProcess) Name() string { return sp.name }

// Context 子进程生命周期 context（实现 strategy.StrategyRuntime）。
func (sp *StrategyProcess) Context() context.Context { return sp.ctx }

// NewStrategyProcess 创建策略进程。
func NewStrategyProcess(ctx context.Context, s *strategy.Strategy) (strategy.StrategyRuntime, error) {
	procCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(procCtx,
		"python3",
		"-u",
		s.ScriptPath,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start strategy: %w", err)
	}

	sp := &StrategyProcess{
		name:      s.Name,
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		heartbeat: time.Now(),
		crashes:   make([]time.Time, 0),
		ctx:       procCtx,
		cancel:    cancel,
	}

	return sp, nil
}

// SendMessage 发送消息到策略进程。
func (sp *StrategyProcess) SendMessage(msg *strategy.IpcMessage) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = sp.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// SendInit 发送初始化消息。
func (sp *StrategyProcess) SendInit(s *strategy.Strategy) error {
	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeInit, map[string]interface{}{
		"strategy_name": s.Name,
		"symbols":       s.Symbols,
		"parameters":    s.Parameters,
	}))
}

// SendTick 发送 ticker 消息。
func (sp *StrategyProcess) SendTick(ticker *models.Ticker, traceID string) error {
	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeTick, map[string]interface{}{
		"trace_id": traceID,
		"ticker":   ticker,
	}))
}

// SendKline 发送收盘 K 线消息。
func (sp *StrategyProcess) SendKline(kline *models.Kline, traceID string) error {
	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeKline, map[string]interface{}{
		"trace_id": traceID,
		"kline":    kline,
	}))
}

// SendHistory 发送 REST 预热历史 K 线。
func (sp *StrategyProcess) SendHistory(payload *models.HistoryPayload) error {
	if payload == nil {
		return nil
	}
	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeHistory, map[string]interface{}{
		"interval": payload.Interval,
		"series":   payload.Series,
	}))
}

// SendStop 发送停止消息。
func (sp *StrategyProcess) SendStop() error {
	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeStop, nil))
}

// SendRPCResponse 发送 RPC 响应。
func (sp *StrategyProcess) SendRPCResponse(requestID string, result interface{}, err error) error {
	data := map[string]interface{}{
		"request_id": requestID,
	}

	if err != nil {
		data["error"] = map[string]interface{}{
			"code":    2000,
			"message": err.Error(),
		}
	} else {
		data["result"] = result
	}

	return sp.SendMessage(strategy.NewIpcMessage(strategy.IpcMessageTypeRPCResponse, data))
}

// ReadMessages 读取策略输出（阻塞）。
func (sp *StrategyProcess) ReadMessages(handler func(strategy.IpcMessage)) error {
	scanner := bufio.NewScanner(sp.stdout)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-sp.ctx.Done():
			return sp.ctx.Err()
		default:
		}

		line := scanner.Text()

		var msg strategy.IpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			logger.ErrorContext(sp.ctx, "Failed to parse message", logger.String("strategy", sp.name), logger.Any("error", err), logger.Any("raw", line))
			continue
		}

		handler(msg)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// ReadStderr 读取 stderr（用于调试）。
func (sp *StrategyProcess) ReadStderr() {
	scanner := bufio.NewScanner(sp.stderr)
	for scanner.Scan() {
		logger.DebugContext(sp.ctx, "receive stderr msg", logger.String("strategy", sp.name), logger.String("text", scanner.Text()))
	}
}

// UpdateHeartbeat 更新心跳时间。
func (sp *StrategyProcess) UpdateHeartbeat() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.heartbeat = time.Now()
}

// GetLastHeartbeat 获取最后心跳时间。
func (sp *StrategyProcess) GetLastHeartbeat() time.Time {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.heartbeat
}

// RecordCrash 记录崩溃。
func (sp *StrategyProcess) RecordCrash() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.crashes = append(sp.crashes, time.Now())
}

// PruneCrashes 剔除滑动窗口外的崩溃记录。
func (sp *StrategyProcess) PruneCrashes(window time.Duration) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.crashes = pruneLocalCrashTimes(sp.crashes, window, time.Now())
}

func pruneLocalCrashTimes(times []time.Time, window time.Duration, now time.Time) []time.Time {
	if window <= 0 || len(times) == 0 {
		return times
	}
	cutoff := now.Add(-window)
	out := times[:0]
	for _, t := range times {
		if !t.Before(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// GetCrashCount 返回当前保留的崩溃次数（应先 PruneCrashes 或使用 Engine 侧计数）。
func (sp *StrategyProcess) GetCrashCount() int {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return len(sp.crashes)
}

// Stop 停止策略进程。
func (sp *StrategyProcess) Stop() error {
	if err := sp.SendStop(); err != nil {
		logger.ErrorContext(sp.ctx, "failed to send stop message", logger.String("strategy", sp.name), logger.Any("error", err))
	}

	time.Sleep(2 * time.Second)

	sp.cancel()

	if err := sp.cmd.Wait(); err != nil {
		return fmt.Errorf("process exit with error: %w", err)
	}

	return nil
}

// Kill 强制杀死进程。
func (sp *StrategyProcess) Kill() error {
	sp.cancel()
	return sp.cmd.Process.Kill()
}

// IsRunning 检查进程是否运行。
func (sp *StrategyProcess) IsRunning() bool {
	if sp.cmd == nil || sp.cmd.Process == nil {
		return false
	}

	select {
	case <-sp.ctx.Done():
		return false
	default:
		return true
	}
}
