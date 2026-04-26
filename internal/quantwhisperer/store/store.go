package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := ensureParent(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			broker TEXT NOT NULL,
			symbol TEXT NOT NULL,
			last_price REAL NOT NULL,
			momentum REAL NOT NULL,
			action TEXT NOT NULL,
			confidence REAL NOT NULL,
			reasoning TEXT NOT NULL,
			source TEXT NOT NULL,
			raw TEXT NOT NULL DEFAULT '',
			raw_response TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS paper_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			broker TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			quantity REAL NOT NULL,
			price REAL NOT NULL,
			confidence REAL NOT NULL,
			reasoning TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS live_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			broker TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			quantity REAL NOT NULL,
			price REAL NOT NULL,
			confidence REAL NOT NULL,
			reasoning TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS pnl_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			mode TEXT NOT NULL,
			symbol TEXT NOT NULL,
			cash REAL NOT NULL,
			position_qty REAL NOT NULL,
			last_price REAL NOT NULL,
			equity REAL NOT NULL,
			drawdown_pct REAL NOT NULL
		);`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	if err := ensureColumn(db, "decisions", "raw_response", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "decisions", "raw", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if hasRaw, err := hasColumn(db, "decisions", "raw"); err == nil && hasRaw {
		if _, err := db.Exec(`UPDATE decisions SET raw_response = raw WHERE raw_response = ''`); err != nil {
			return fmt.Errorf("migration failed while copying raw -> raw_response: %w", err)
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table string, column string, ddl string) error {
	exists, err := hasColumn(db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	statement := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, ddl)
	if _, err := db.Exec(statement); err != nil {
		return fmt.Errorf("migration failed while adding %s.%s: %w", table, column, err)
	}
	return nil
}

func hasColumn(db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, fmt.Errorf("migration failed while reading table info for %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid      int
			name     string
			typeName string
			notNull  int
			defaultV sql.NullString
			pk       int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultV, &pk); err != nil {
			return false, fmt.Errorf("migration failed while scanning table info for %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("migration failed while iterating table info for %s: %w", table, err)
	}
	return false, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) LogDecision(tick quantwhisperer.Tick, momentum float64, decision quantwhisperer.Decision, source string) error {
	_, err := s.db.Exec(
		`INSERT INTO decisions (created_at, broker, symbol, last_price, momentum, action, confidence, reasoning, source, raw, raw_response)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tick.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		tick.Broker,
		tick.Symbol,
		tick.LastPrice,
		momentum,
		decision.Action,
		decision.Confidence,
		decision.Reasoning,
		source,
		decision.Raw,
		decision.Raw,
	)
	return err
}

func (s *Store) LogTrade(trade quantwhisperer.Trade) error {
	table := "paper_trades"
	if trade.Mode == quantwhisperer.ModeLive {
		table = "live_trades"
	}
	query := fmt.Sprintf(`INSERT INTO %s (created_at, broker, symbol, side, quantity, price, confidence, reasoning)
	 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, table)
	_, err := s.db.Exec(
		query,
		trade.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		trade.Broker,
		trade.Symbol,
		trade.Side,
		trade.Quantity,
		trade.Price,
		trade.Confidence,
		trade.Reasoning,
	)
	return err
}

func (s *Store) LogSnapshot(mode quantwhisperer.Mode, snapshot quantwhisperer.Snapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO pnl_history (created_at, mode, symbol, cash, position_qty, last_price, equity, drawdown_pct)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		string(mode),
		snapshot.Symbol,
		snapshot.Cash,
		snapshot.PositionQty,
		snapshot.LastPrice,
		snapshot.Equity,
		snapshot.DrawdownPct,
	)
	return err
}
