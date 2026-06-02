package engine

import "github.com/kainhuck/signalix/internal/app/instrument"

// ContractMetaLookup 返回进程内合约元数据注册表（CM-2/CM-3 注入用）。
func (e *Engine) ContractMetaLookup() instrument.ContractMetaLookup {
	if e == nil || e.metaLookup == nil {
		return instrument.NewRegistry()
	}
	return e.metaLookup
}
