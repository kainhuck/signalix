package instrument

import "github.com/kainhuck/signalix/pkg/exchange/perp"

// ContractMetaLookup 按合约查询元数据（进程内只读）。
type ContractMetaLookup interface {
	ContractMeta(contract perp.Contract) (*perp.ContractMeta, error)
}
