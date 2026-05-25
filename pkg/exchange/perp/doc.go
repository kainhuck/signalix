// Package perp 定义 USDT 本位永续合约的领域类型与细粒度接口，
// 供各交易所适配器实现。量化策略应依赖本包接口而非具体交易所包。
//
// 合约标识 Contract 的规范写法为 BASE/QUOTE（例如 BTC/USDT）；适配层负责与
// 各所原生符号（如 Gate 的 BASE_QUOTE）互转。
package perp
