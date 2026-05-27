package oms

import (
	"time"

	"github.com/kainhuck/signalix/internal/app/instrument"
	"github.com/kainhuck/signalix/internal/models"
)

// Option 执行引擎可选配置。
type Option func(*ExecutionEngine)

// WithChannelBuffers 设置 orderUpdateCh 与 cmdCh 容量。
func WithChannelBuffers(orderUpdate, cmd int) Option {
	return func(e *ExecutionEngine) {
		if e == nil {
			return
		}
		if orderUpdate > 0 {
			e.orderUpdateCh = make(chan *models.Order, orderUpdate)
		}
		if cmd > 0 {
			e.cmdCh = make(chan *omsCmd, cmd)
		}
	}
}

// WithMaxRetries 设置提交失败最大重试次数。
func WithMaxRetries(n int) Option {
	return func(e *ExecutionEngine) {
		if e != nil && n >= 0 {
			e.maxRetries = n
		}
	}
}

// WithRetryInterval 设置重试间隔。
func WithRetryInterval(d time.Duration) Option {
	return func(e *ExecutionEngine) {
		if e != nil && d > 0 {
			e.retryInterval = d
		}
	}
}

// WithContractMetaLookup 注入进程内合约元数据 lookup（Place 前校验）。
func WithContractMetaLookup(lookup instrument.ContractMetaLookup) Option {
	return func(e *ExecutionEngine) {
		if e != nil {
			e.metaLookup = lookup
		}
	}
}
