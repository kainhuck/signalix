package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/kainhuck/signalix/internal/models"
	"github.com/kainhuck/signalix/internal/ports"
)

const schemaVersion = 2

// Store SQLite 实现的 OrderStore 与 PersistenceStore。
type Store struct {
	db *sql.DB
}

var (
	_ ports.OrderStore       = (*Store)(nil)
	_ ports.PersistenceStore = (*Store)(nil)
)

// Open 打开或创建 SQLite 库，启用 WAL，并确保表结构存在。
func Open(path string, maxOpenConns int) (*Store, error) {
	path, err := expandPath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if maxOpenConns <= 0 {
		maxOpenConns = 1
	}
	db.SetMaxOpenConns(maxOpenConns)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("wal pragma: %w", err)
	}
	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func expandPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty database path")
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p), nil
}

func (s *Store) initSchema() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
		v = 1
	}
	if v < 2 {
		if err := s.migrateV2(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateV1() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS orders (
  id TEXT PRIMARY KEY,
  strategy_name TEXT NOT NULL,
  symbol TEXT NOT NULL,
  side TEXT NOT NULL,
  order_type TEXT NOT NULL,
  size TEXT NOT NULL,
  filled_size TEXT NOT NULL,
  status TEXT NOT NULL,
  price TEXT,
  stop_price TEXT,
  exchange_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS strategy_state (
  strategy_name TEXT NOT NULL,
  state_key TEXT NOT NULL,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (strategy_name, state_key)
);
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, 1))
	return err
}

func (s *Store) migrateV2() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS account_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_at TEXT NOT NULL,
  revision INTEGER NOT NULL,
  currency TEXT NOT NULL,
  total_equity TEXT NOT NULL,
  balance_json TEXT NOT NULL,
  positions_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_account_snapshots_snapshot_at
  ON account_snapshots (snapshot_at);

CREATE TABLE IF NOT EXISTS strategy_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  strategy_name TEXT NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_strategy_logs_strategy_created
  ON strategy_logs (strategy_name, created_at);
CREATE INDEX IF NOT EXISTS idx_strategy_logs_created_at
  ON strategy_logs (created_at);
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion))
	return err
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SaveOrder UPSERT 订单行。
func (s *Store) SaveOrder(ctx context.Context, o *models.Order) error {
	if o == nil {
		return fmt.Errorf("nil order")
	}
	var price, stopPrice sql.NullString
	if o.Price != nil {
		price = sql.NullString{String: *o.Price, Valid: true}
	}
	if o.StopPrice != nil {
		stopPrice = sql.NullString{String: *o.StopPrice, Valid: true}
	}
	exID := sql.NullString{String: o.ExchangeID, Valid: o.ExchangeID != ""}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO orders (id, strategy_name, symbol, side, order_type, size, filled_size, status, price, stop_price, exchange_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  strategy_name = excluded.strategy_name,
  symbol = excluded.symbol,
  side = excluded.side,
  order_type = excluded.order_type,
  size = excluded.size,
  filled_size = excluded.filled_size,
  status = excluded.status,
  price = excluded.price,
  stop_price = excluded.stop_price,
  exchange_id = excluded.exchange_id,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at
`,
		o.ID, o.StrategyName, string(o.Symbol), string(o.Side), string(o.OrderType),
		o.Size, o.FilledSize, string(o.Status), price, stopPrice, exID,
		o.CreatedAt.UTC().Format(time.RFC3339Nano),
		o.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// ListNonTerminalOrders 列出非终态订单。
func (s *Store) ListNonTerminalOrders(ctx context.Context) ([]*models.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, strategy_name, symbol, side, order_type, size, filled_size, status, price, stop_price, exchange_id, created_at, updated_at
FROM orders
WHERE status IN ('Pending','Submitted','PartialFilled')
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func scanOrder(sc interface {
	Scan(dest ...any) error
}) (*models.Order, error) {
	var (
		id, strat, sym, side, otype, size, filled, status string
		price, stopPrice, exID                            sql.NullString
		createdS, updatedS                                string
	)
	if err := sc.Scan(&id, &strat, &sym, &side, &otype, &size, &filled, &status, &price, &stopPrice, &exID, &createdS, &updatedS); err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdS)
	if err != nil {
		createdAt, _ = time.Parse(time.RFC3339, createdS)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedS)
	if err != nil {
		updatedAt, _ = time.Parse(time.RFC3339, updatedS)
	}
	o := &models.Order{
		ID:           id,
		StrategyName: strat,
		Symbol: sym,
		Side:         models.OrderSide(side),
		OrderType:    models.OrderType(otype),
		Size:         size,
		FilledSize:   filled,
		Status:       models.OrderStatus(status),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if exID.Valid {
		o.ExchangeID = exID.String
	}
	if price.Valid {
		p := price.String
		o.Price = &p
	}
	if stopPrice.Valid {
		p := stopPrice.String
		o.StopPrice = &p
	}
	return o, nil
}

// SaveStrategyState UPSERT 策略自定义状态。
func (s *Store) SaveStrategyState(ctx context.Context, strategyName, stateKey string, valueJSON []byte, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO strategy_state (strategy_name, state_key, value_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(strategy_name, state_key) DO UPDATE SET
  value_json = excluded.value_json,
  updated_at = excluded.updated_at
`, strategyName, stateKey, string(valueJSON), updatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

// LoadAllStrategyStates 加载全部策略状态；内层 map 值为原始 JSON 字节，由调用方反序列化。
func (s *Store) LoadAllStrategyStates(ctx context.Context) (map[string]map[string][]byte, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT strategy_name, state_key, value_json FROM strategy_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[string][]byte)
	for rows.Next() {
		var sn, key, vj string
		if err := rows.Scan(&sn, &key, &vj); err != nil {
			return nil, err
		}
		if out[sn] == nil {
			out[sn] = make(map[string][]byte)
		}
		out[sn][key] = []byte(vj)
	}
	return out, rows.Err()
}
