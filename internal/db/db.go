package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct {
	*sqlx.DB
}

// Open opens (or creates) the SQLite database and sets WAL mode.
// Schema management is handled by goose migrations, which run inside
// `waypoint start` (see RunMigrations). This function does NOT create
// tables — callers that need schema must ensure migrations have run.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Do NOT set this to 1. With a single connection, any code path that
	// checks out a connection and fails to return it (a leaked *sql.Rows
	// whose Close is skipped, an uncommitted tx, a goroutine that died
	// mid-query) permanently deadlocks the entire database. A small pool
	// (4) gives enough headroom that a single leak degrades performance
	// instead of wedging the server, while staying low enough that SQLite
	// writer contention is rare (WAL + busy_timeout=5000 serializes
	// writers at the file level anyway).
	db.DB.SetMaxOpenConns(4)

	// PRAGMAs: WAL + tuning for a local single-user database.
	// journal_mode=WAL is persistent (stored in DB header).
	// The rest are per-connection and must be set on every Open.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA cache_size=-20000"); err != nil {
		return nil, fmt.Errorf("set cache size: %w", err)
	}
	if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
		return nil, fmt.Errorf("set temp store: %w", err)
	}

	return &Store{db}, nil
}

// tx runs a function inside a transaction.
func (s *Store) tx(fn func(*sqlx.Tx) error) error {
	tx, err := s.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.DB.Close()
}
