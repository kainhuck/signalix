# AGENTS.md — Signalix

## Quick reference

```bash
make run          # go run ./cmd/signalixd
make build        # build daemon to bin/signalixd
make build-cli    # build CLI to bin/signalix
make test         # go test ./...
make proto        # regenerate api/gen/go/ from api/proto/ (buf)
make fmt          # go fmt ./...
make tidy         # go mod tidy
make clean        # rm -rf bin/
```

## Architecture

**Two binaries** in one Go module (`github.com/kainhuck/signalix`, `go 1.25.0`):

| Binary | Entry | Purpose |
|--------|-------|---------|
| `signalixd` | `cmd/signalixd/` | Daemon engine: market data, strategy IPC, OMS, risk, optional gRPC server |
| `signalix` | `cmd/signalix/` | CLI client talking to `signalixd` over gRPC |

**Strict ports & adapters layering:**

```
internal/domain/risk/   # Pure functions, zero deps (Evaluate / Verdict)
internal/ports/         # Interfaces: Exchange, OrderStore, PersistenceStore, RiskEvaluator
internal/app/           # Application logic: engine, market, oms, decision, strategy, projection
internal/adapters/      # Implementations: gateio, pythonipc, sqlite, grpc
pkg/exchange/perp/      # Perp DTOs and perp.Live interface + gateio adapter
internal/models/        # Market-neutral views, signals, orders, klines
```

- `internal/domain` MUST NOT import IO packages.
- `internal/app` depends on `internal/ports`, never on `internal/adapters` directly.
- `internal/adapters` implements `internal/ports`.

**Only `perp` market is supported.** `spot` is listed in config but will cause a hard startup failure.

## Proto codegen

- Protos live in `api/proto/signalix/` with `buf.gen.yaml` and `buf.yaml`.
- Generated Go code lands in `api/gen/go/` (committed, not `.gitignore`d).
- Regenerate after editing protos: `make proto` (runs `buf generate` from `api/proto/` with explicit `GOPROXY=…`).
- Also run `make tidy` afterwards if new imports appear.

## Tests

```bash
go test ./...                  # all tests
go test ./internal/app/engine/ # single package
go test ./... -run TestFoo     # single test by name
```

- No external services needed; tests use `internal/testutil.StubExchange` to fake the exchange.
- Test helpers live alongside their package (e.g. `engine_test_helpers_test.go` in `internal/app/engine/`).
- `test/` directory is **gitignored** and contains local-only dev scripts with hardcoded credentials — ignore it.

## Configuration

- Work directory: `SIGNALIX_WORK_DIR` env var (default `~/.signalix`).
- Global config: `$SIGNALIX_WORK_DIR/config.toml` (copy from `config.example.toml`).
- CLI config: `$SIGNALIX_WORK_DIR/cli.toml` (copy from `config.example.cli.toml`).
- Strategy config: per-directory `config.yaml` under `$SIGNALIX_WORK_DIR/strategies/<name>/`.
- SQLite path: always `$SIGNALIX_WORK_DIR/data/signalix.db` (hardcoded, not in TOML).
- gRPC is optional; set `[grpc] enabled = true` in `config.toml` to expose the control plane on `127.0.0.1:50051`.

## Python strategies

- One Python process per strategy, communicating via **stdin/stdout JSON**.
- SDK lives at `sdk/python/` (install with `pip install -e ./sdk/python`).
- Each strategy directory needs both `strategy.py` and `config.yaml`.
- Strategy template scaffolding is embedded in `internal/app/strategy/scaffold/templates/`.

## Gotchas

- **Do not** import `internal/adapters` from `internal/app` or `internal/domain`.
- After editing protos or go.mod, run `make proto && make tidy`.
- The `gateio` adapter in `pkg/exchange/perp/gateio/` is the **only** exchange backend. Tests for it need no real credentials.
- Config keys use snake_case in TOML; Go structs use pascal-case mapping via viper.
- The engine can run without gRPC — it just blocks on SIGINT/SIGTERM and runs strategies locally.
- Kill switch: gRPC `ActivateKillSwitch` or `signalix kill-switch activate`. Config field is `risk.kill_switch.cancel_open_orders_on_activate`.

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Finding relationships**: `query_graph` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview` + `list_communities`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context` | Need source snippets for review — token-efficient |
| `get_impact_radius` | Understanding blast radius of a change |
| `get_affected_flows` | Finding which execution paths are impacted |
| `query_graph` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes` | Finding functions/classes by name or keyword |
| `get_architecture_overview` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes` for code review.
3. Use `get_affected_flows` to understand impact.
4. Use `query_graph` pattern="tests_for" to check coverage.
