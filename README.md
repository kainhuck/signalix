<p align="center">
  <img src="docs/assets/logo.png" alt="Signalix — local-first perpetual futures trading engine" width="420">
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go 1.25+"></a>
</p>

<p align="center">
  <b>English</b> · <a href="README.zh-CN.md">简体中文</a>
</p>

**Local-first crypto perpetual futures trading engine** — Go core + Python strategy subprocesses, with [Gate.io](https://www.gate.io/) as the first supported exchange (testnet & live).

Signalix orchestrates market data, account projection, decision-making, risk checks, OMS, and strategy lifecycle in a single Go process. Strategy logic runs in Python and talks to the engine over **stdin/stdout JSON**. An optional **gRPC** control plane supports automation and future HTTP gateway integration.

> **Risk disclaimer:** Trading involves substantial risk of loss. This project is for learning and research only and does not constitute investment advice. Complete your own security, compliance, and risk review before live trading. Never commit API keys to version control.

---

## Features

- **Perpetual abstraction** (`pkg/exchange/perp`): unified connect, market data, trading, account views, and user streams
- **Gate.io adapter**: REST / WebSocket with testnet and live configuration
- **Multi-strategy isolation**: one Python process per strategy; a single crash does not take down the engine
- **Full trading pipeline**: MarketRouter → strategy IPC → DecisionEngine → risk → OMS
- **K-line driven**: closed-bar IPC, REST `on_history` warmup, strategy RPCs (`get_balance` / `get_position` / `get_klines`)
- **Account projection**: REST refresh + private WS patches to cut REST on the hot path
- **Local persistence**: SQLite for open orders and strategy `set_state`; reconcile on startup
- **Optional gRPC**: `signalix.engine.v1.Engine` (start/stop strategies, query orders, order event stream)

## Architecture

```
┌──────────────┐   optional   ┌─────────────────┐
│ CLI / bots   │ ── gRPC ──► │  signalixd      │
└──────────────┘              │  (Go engine)    │
                              └────────┬────────┘
                                       │
         ┌─────────────────────────────┼─────────────────────────────┐
         ▼                             ▼                             ▼
  MarketRouter               Python strategy(ies)              ExecutionEngine
  AccountProjection          stdin/stdout IPC                       (OMS)
         │                             │                             │
         └─────────────────────────────┴─────────────────────────────┘
                                       │
                                       ▼
                             Gate.io perpetual (REST / WS)
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/PRODUCT_DESIGN.md](docs/PRODUCT_DESIGN.md) for design details *(currently in Chinese)*.

## Quick start

### Requirements

- Go **1.25+** (see `go.mod`)
- Python **3.8+**
- Gate.io API credentials (testnet or live)

### Install & configure

```bash
git clone https://github.com/kainhuck/signalix.git
cd signalix

pip install -e ./sdk/python

mkdir -p ~/.signalix/data ~/.signalix/strategies
cp config.example.toml ~/.signalix/config.toml
# Edit exchange.api_key / api_secret / user_id in ~/.signalix/config.toml

# Example strategies (symlink or copy)
ln -s "$(pwd)/sdk/python/example" ~/.signalix/strategies/example
```

The work directory is set by `SIGNALIX_WORK_DIR` (default: `~/.signalix`):

| Path | Description |
|------|-------------|
| `config.toml` | Global configuration |
| `data/signalix.db` | SQLite database |
| `strategies/` | Strategy root directory |

### Run

```bash
go run ./cmd/signalixd
# or
make build && ./bin/signalix
```

Enable gRPC (`[grpc] enabled = true` in `config.toml`):

```bash
grpcurl -plaintext -d '{}' localhost:50051 signalix.engine.v1.Engine/Ping
```

Regenerate after editing protos:

```bash
make proto
```

## Writing strategies

Each strategy lives in its own subdirectory with both `strategy.py` and `config.yaml`:

```
strategies/
└── my_strategy/
    ├── strategy.py
    └── config.yaml
```

Example `config.yaml`:

```yaml
name: example_trend
enabled: true
symbols:
  - BTC/USDT
interval: 1m
history_bars: 200
parameters:
  ma_fast: 10
  ma_slow: 30
```

See [sdk/python/README.md](sdk/python/README.md) for the Python SDK and `sdk/python/example/` for samples.

## Repository layout

```
signalix/
├── api/proto/              # gRPC contracts (Buf)
├── api/gen/go/             # Generated Go code
├── cmd/signalixd/          # Engine entrypoint
├── internal/
│   ├── app/                # engine, oms, projection, market, decision, strategy
│   ├── adapters/           # gateio, pythonipc, sqlite, grpc
│   ├── domain/risk/        # Risk rules
│   ├── ports/              # Port interfaces
│   └── models/
├── pkg/exchange/perp/      # Perpetual domain model & Gate.io implementation
├── sdk/python/             # Python SDK
├── config.example.toml
├── docs/                   # Architecture & product design
└── templates/              # Strategy templates
```

## Known limitations

These are partially stubbed in config or docs — read [config.example.toml](config.example.toml) before relying on them:

- Some risk fields (`max_daily_loss`, `max_leverage`, etc.) are in config but not fully enforced yet
- Engine-side auto-restart after strategy crashes
- HTTP gateway (UI layer, separate from this repo)
- Exchange REST `rate_limit` config is not wired to a limiter yet

## Roadmap

- Live-trading hardening: auto-restart, full risk rules, kill switch
- HTTP gateway (OpenAPI + auth, backed by engine gRPC)
- More exchanges, contract metadata cache, backtest / paper mode

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Product design](docs/PRODUCT_DESIGN.md)
- [Python SDK](sdk/python/README.md)

## Contributing

Issues and pull requests are welcome:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes
4. Open a pull request

Run `go test ./...` before submitting. If you change protos, run `make proto` and commit generated code.

## License

[MIT License](LICENSE)

## Links

- **GitHub**: https://github.com/kainhuck/signalix
- **Issues**: https://github.com/kainhuck/signalix/issues
