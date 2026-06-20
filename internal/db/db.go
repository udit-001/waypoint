package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// schemaStmts holds the DDL statements executed in order on database creation.
// Each entry is a single SQL statement (no multi-statement blocks) so that
// the pure-Go SQLite driver can parse trigger BEGIN…END blocks correctly.
var schemaStmts = []string{
	`CREATE TABLE IF NOT EXISTS categories (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE)`,

	`CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		company TEXT NOT NULL DEFAULT '',
		position TEXT NOT NULL DEFAULT '',
		date TEXT NOT NULL DEFAULT '',
		applied_date TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'Not Applied',
		category_id INTEGER NOT NULL DEFAULT 1 REFERENCES categories(id),
		salary TEXT NOT NULL DEFAULT '',
		location TEXT NOT NULL DEFAULT '',
		contact TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		notes TEXT NOT NULL DEFAULT '',
		reminder_date TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
		action TEXT NOT NULL,
		from_value TEXT NOT NULL DEFAULT '',
		to_value TEXT NOT NULL DEFAULT '',
		timestamp TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	`CREATE TABLE IF NOT EXISTS profile (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		name TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		phone TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		skills TEXT NOT NULL DEFAULT '[]',
		experience TEXT NOT NULL DEFAULT '[]',
		education TEXT NOT NULL DEFAULT '[]',
		industry TEXT NOT NULL DEFAULT '',
		greeting_style TEXT NOT NULL DEFAULT 'formal',
		sign_off TEXT NOT NULL DEFAULT 'Best regards'
	)`,

	`INSERT OR IGNORE INTO profile (id) VALUES (1)`,

	`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		theme TEXT NOT NULL DEFAULT 'light',
		reminders_enabled INTEGER NOT NULL DEFAULT 1,
		default_view TEXT NOT NULL DEFAULT 'dashboard',
		items_per_page INTEGER NOT NULL DEFAULT 25
	)`,

	`INSERT OR IGNORE INTO settings (id) VALUES (1)`,

	// Jobs FTS5 full-text search index
	`CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(company, position, notes, location, contact, category, content=jobs, content_rowid=id)`,

	`CREATE TRIGGER IF NOT EXISTS jobs_ai AFTER INSERT ON jobs BEGIN
		INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
		VALUES (new.id, new.company, new.position, new.notes, new.location, new.contact, (SELECT name FROM categories WHERE id = new.category_id));
	END`,

	`CREATE TRIGGER IF NOT EXISTS jobs_ad AFTER DELETE ON jobs BEGIN
		INSERT INTO jobs_fts(jobs_fts, rowid, company, position, notes, location, contact, category)
		VALUES('delete', old.id, old.company, old.position, old.notes, old.location, old.contact, (SELECT name FROM categories WHERE id = old.category_id));
	END`,

	`CREATE TRIGGER IF NOT EXISTS jobs_au AFTER UPDATE ON jobs BEGIN
		INSERT INTO jobs_fts(jobs_fts, rowid, company, position, notes, location, contact, category)
		VALUES('delete', old.id, old.company, old.position, old.notes, old.location, old.contact, (SELECT name FROM categories WHERE id = old.category_id));
		INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
		VALUES (new.id, new.company, new.position, new.notes, new.location, new.contact, (SELECT name FROM categories WHERE id = new.category_id));
	END`,

	`CREATE TABLE IF NOT EXISTS artifacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id TEXT NOT NULL,
		job_id INTEGER REFERENCES jobs(id) ON DELETE SET NULL,
		title TEXT NOT NULL DEFAULT '',
		options TEXT NOT NULL DEFAULT '{}',
		variants TEXT NOT NULL DEFAULT '[]',
		archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,

	// Artifacts FTS5 full-text search index
	`CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(title, skill_id, content=artifacts, content_rowid=id)`,

	`CREATE TRIGGER IF NOT EXISTS artifacts_ai AFTER INSERT ON artifacts BEGIN
		INSERT INTO artifacts_fts(rowid, title, skill_id) VALUES (new.id, new.title, new.skill_id);
	END`,

	`CREATE TRIGGER IF NOT EXISTS artifacts_ad AFTER DELETE ON artifacts BEGIN
		INSERT INTO artifacts_fts(artifacts_fts, rowid, title, skill_id) VALUES('delete', old.id, old.title, old.skill_id);
	END`,

	`CREATE TRIGGER IF NOT EXISTS artifacts_au AFTER UPDATE ON artifacts BEGIN
		INSERT INTO artifacts_fts(artifacts_fts, rowid, title, skill_id) VALUES('delete', old.id, old.title, old.skill_id);
		INSERT INTO artifacts_fts(rowid, title, skill_id) VALUES (new.id, new.title, new.skill_id);
	END`,
}

const seedCategories = `
INSERT OR IGNORE INTO categories (name) VALUES ('General');
INSERT OR IGNORE INTO categories (name) VALUES ('Tech'), ('Finance'), ('Healthcare');
`

// Store wraps the SQLite database.
type Store struct {
	*sqlx.DB
}

// Open opens (or creates) the SQLite database, sets WAL mode, and runs migrations.
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

	db.DB.SetMaxOpenConns(1)

	// Enable WAL mode for concurrent reads + safe single writer
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Run schema migration (statement by statement for trigger compatibility)
	for i, stmt := range schemaStmts {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("run schema stmt %d: %w", i, err)
		}
	}
	if _, err := db.Exec(seedCategories); err != nil {
		return nil, fmt.Errorf("seed categories: %w", err)
	}

	// Rebuild FTS indices to ensure existing data is indexed
	store := &Store{db}
	_ = store.RebuildFTS()

	return store, nil
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

// nullString returns a *string for a potential empty string (used for db scan).
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// valueString returns the string value, or "" if nil.
func valueString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ensureDB ensures the DB is reachable.
func (s *Store) EnsureDB() error {
	return s.DB.Ping()
}

// exists checks if a row exists in a table by id.
func (s *Store) exists(table string, id int64) (bool, error) {
	var count int
	err := s.Get(&count, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", table), id)
	return count > 0, err
}

// EmptyDB deletes all data from all tables (for testing/reset).
func (s *Store) EmptyDB() error {
	_, err := s.Exec("DELETE FROM history")
	if err != nil {
		return err
	}
	_, err = s.Exec("DELETE FROM jobs")
	if err != nil {
		return err
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.DB.Close()
}

// RebuildFTS rebuilds the FTS indices from scratch. Call this after upgrading
// from a version that didn't have FTS tables.
func (s *Store) RebuildFTS() error {
	// Rebuild jobs FTS
	if _, err := s.Exec("INSERT INTO jobs_fts(jobs_fts) VALUES('rebuild')"); err != nil {
		// Table may not exist yet; that's OK — the schema will create it on next Open.
		return nil
	}
	// Rebuild artifacts FTS
	if _, err := s.Exec("INSERT INTO artifacts_fts(artifacts_fts) VALUES('rebuild')"); err != nil {
		return nil
	}
	return nil
}

var _ sql.DB // ensure import is used (the blank import handles the driver)
