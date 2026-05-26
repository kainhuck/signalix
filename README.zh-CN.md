<p align="center">
  <img src="docs/assets/logo.png" alt="Signalix — 本地化永续合约量化引擎" width="420">
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache 2.0"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go 1.25+"></a>
</p>

<p align="center">
  <a href="README.md">English</a> · <b>简体中文</b>
</p>

**本地化加密货币永续合约量化引擎** — Go 核心 + Python 策略子进程，一期优先适配 [Gate.io](https://www.gate.io/)（含模拟盘）。

Signalix 将行情接入、账户投影、决策、风控、OMS 与策略生命周期编排在一个 Go 进程中；策略逻辑用 Python 编写，通过 **stdin/stdout JSON** 与引擎通信。可选 **gRPC** 控制面供自动化或后续 HTTP 网关集成。

> **风险提示**：量化交易存在资金损失风险。本项目用于学习与研究，不构成投资建议。接入实盘前请自行完成安全、合规与风控评估；切勿将 API 密钥提交到版本库。

---

## 特性

- **永续抽象层**（`pkg/exchange/perp`）：统一连接、行情、交易、账户视图与用户推送接口
- **Gate.io 适配**：REST / WebSocket，支持模拟盘与实盘配置切换
- **多策略隔离**：每策略独立 Python 进程；单策略崩溃不拖垮引擎
- **完整交易管线**：MarketRouter → 策略 IPC → DecisionEngine → 风控 → OMS
- **K 线驱动**：收盘 K 线 IPC、`on_history` REST 预热、策略侧 RPC（`get_balance` / `get_position` / `get_klines`）
- **账户投影**：REST 校准 + 私有 WS 增量，决策路径减少 REST 往返
- **本地持久化**：SQLite 保存未完成订单与策略 `set_state`；启动时对账
- **可选 gRPC**：`signalix.engine.v1.Engine`（启停策略、查单、订单事件流等）

## 架构概览

```
┌──────────────┐     可选      ┌─────────────────┐
│ CLI / 自动化  │ ── gRPC ──► │  signalixd      │
└──────────────┘               │  (Go Engine)    │
                               └────────┬────────┘
                                        │
          ┌─────────────────────────────┼─────────────────────────────┐
          ▼                             ▼                             ▼
   MarketRouter              Python Strategy(s)                  ExecutionEngine
   AccountProjection         stdin/stdout IPC                    (OMS)
          │                             │                             │
          └─────────────────────────────┴─────────────────────────────┘
                                        │
                                        ▼
                              Gate.io Perpetual (REST / WS)
```

更完整的设计说明见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) 与 [docs/PRODUCT_DESIGN.md](docs/PRODUCT_DESIGN.md)。

## 快速开始

### 环境要求

- Go **1.25+**（见 `go.mod`）
- Python **3.8+**
- Gate.io API 密钥（模拟盘或实盘）

### 安装与配置

```bash
git clone https://github.com/kainhuck/signalix.git
cd signalix

pip install -e ./sdk/python

mkdir -p ~/.signalix/data ~/.signalix/strategies
cp config.example.toml ~/.signalix/config.toml
# 编辑 ~/.signalix/config.toml 中的 exchange.api_key / api_secret / user_id

# 示例策略（软链或复制）
ln -s "$(pwd)/sdk/python/example" ~/.signalix/strategies/example
```

工作目录由环境变量 `SIGNALIX_WORK_DIR` 指定，默认为 `~/.signalix`：

| 路径 | 说明 |
|------|------|
| `config.toml` | 全局配置 |
| `data/signalix.db` | SQLite 数据库 |
| `strategies/` | 策略根目录 |

### 运行

```bash
go run ./cmd/signalixd
# 或
make build && ./bin/signalix
```

启用 gRPC（在 `config.toml` 中 `[grpc] enabled = true`）：

```bash
grpcurl -plaintext -d '{}' localhost:50051 signalix.engine.v1.Engine/Ping

# Readiness（GR-1）
grpcurl -plaintext -d '{}' localhost:50051 signalix.engine.v1.Engine/GetHealth
grpcurl -plaintext -d '{"skip_exchange_ping":true}' \
  localhost:50051 signalix.engine.v1.Engine/GetHealth

# 账户只读（GR-2）
grpcurl -plaintext -d '{}' localhost:50051 signalix.engine.v1.Engine/GetBalance
grpcurl -plaintext -d '{"symbol":"BTC/USDT"}' \
  localhost:50051 signalix.engine.v1.Engine/GetPosition
grpcurl -plaintext -d '{}' localhost:50051 signalix.engine.v1.Engine/ListPositions

# Kill Switch（EH-3）
grpcurl -plaintext -d '{"reason":"manual drill"}' \
  localhost:50051 signalix.engine.v1.Engine/ActivateKillSwitch
grpcurl -plaintext -d '{}' \
  localhost:50051 signalix.engine.v1.Engine/GetKillSwitchStatus
grpcurl -plaintext -d '{}' \
  localhost:50051 signalix.engine.v1.Engine/DeactivateKillSwitch
```

修改 Proto 后重新生成：

```bash
make proto
```

## 编写策略

每个策略占一个子目录，需同时包含 `strategy.py` 与 `config.yaml`：

```
strategies/
└── my_strategy/
    ├── strategy.py
    └── config.yaml
```

`config.yaml` 示例：

```yaml
name: example_trend
enabled: true
symbols:
  - BTC/USDT
interval: 1m
history_bars: 200
# 可选 risk 段：策略级限额（不写则仅用 config.toml [risk]）
# risk:
#   max_open_orders: 10
parameters:
  ma_fast: 10
  ma_slow: 30
```

Python SDK 用法见 [sdk/python/README.md](sdk/python/README.md)，示例见 `sdk/python/example/`。

## 仓库结构

```
signalix/
├── api/proto/              # gRPC 契约（Buf）
├── api/gen/go/             # 生成的 Go 代码
├── cmd/signalixd/          # 引擎入口
├── internal/
│   ├── app/                # engine、oms、projection、market、decision、strategy
│   ├── adapters/           # gateio、pythonipc、sqlite、grpc
│   ├── domain/risk/        # 风控规则
│   ├── ports/              # 端口接口
│   └── models/
├── pkg/exchange/perp/      # 永续领域模型与 Gate.io 实现
├── sdk/python/             # Python SDK
├── config.example.toml
├── docs/                   # 架构与产品设计
└── templates/              # 策略模板
```

## 已知限制

以下能力在配置或文档中已有占位，**尚未完全接入**，使用前请阅读 [config.example.toml](config.example.toml) 注释：

- HTTP 网关（UI 对接层，独立于本仓库）
- 交易所 REST `rate_limit` 配置项尚未接入限流器

全局风控（`[risk]`）已支持锁仓、日亏、回撤、杠杆、单合约名义上限等；策略可在 `config.yaml` 可选 `risk` 段配置更紧限额（EH-4）。Kill Switch 见 gRPC；策略崩溃后可在 `[strategies.restart]` 配置自动重启。

## 路线图

- HTTP 网关（OpenAPI + 认证，调用引擎 gRPC）
- 更多交易所适配、合约元数据缓存、回测 / Paper 模式

## 文档

- [技术架构](docs/ARCHITECTURE.md)
- [产品设计纲要](docs/PRODUCT_DESIGN.md)
- [Python SDK](sdk/python/README.md)

## 参与贡献

欢迎 Issue 与 Pull Request：

1. Fork 本仓库
2. 创建特性分支（`git checkout -b feature/your-feature`）
3. 提交变更（`git commit -m 'Add something'`）
4. 推送分支并发起 Pull Request

提交 PR 前请确保 `go test ./...` 通过；若修改 Proto，请运行 `make proto` 并一并提交生成代码。

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源。

## 链接

- **GitHub**: https://github.com/kainhuck/signalix
- **Issues**: https://github.com/kainhuck/signalix/issues
