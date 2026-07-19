package db

import (
	"embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// baselinedb marks V1 as applied in a single transaction so a crash
// mid-baseling rolls back cleanly — no half-created goose_db_version.
func baselinedb(db *sqlx.DB) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin baselining transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp DATETIME DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create goose_db_version: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX goose_db_version_version_id_is_applied_idx ON goose_db_version (version_id, is_applied)`); err != nil {
		return fmt.Errorf("create goose_db_version index: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES (1, 1, datetime('now'))`); err != nil {
		return fmt.Errorf("baseline V1: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit baselining: %w", err)
	}
	return nil
}

// RunMigrations runs pending goose migrations on the database.
//
// For existing databases that predate goose (tables exist but
// goose_db_version does not), V1 is marked as applied without running
// its SQL — the schema is already in place. V2 and any later migrations
// then run normally.
//
// For fresh databases, goose creates goose_db_version and runs all
// migrations from V1 onward.
func (s *SQLiteStore) RunMigrations(dbPath string) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	// Baselining: check whether goose_db_version already exists.
	var gooseTableExists bool
	if err := s.Get(&gooseTableExists,
		`SELECT count(*) > 0 FROM sqlite_master WHERE type='table' AND name='goose_db_version'`); err != nil {
		return fmt.Errorf("check goose_db_version existence: %w", err)
	}

	if !gooseTableExists {
		// Does the database already have application tables?
		var jobsTableCount int
		if err := s.Get(&jobsTableCount,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='jobs'`); err != nil {
			return fmt.Errorf("check jobs table existence: %w", err)
		}

		if jobsTableCount > 0 {
			// Existing DB — baseline V1 as applied so goose skips it.
			// Wrapped in a transaction so a crash mid-baseling rolls back
			// cleanly — no half-created goose_db_version table.
			if err := baselinedb(s.DB); err != nil {
				return err
			}
		}
		// Fresh DB: goose.Up will create goose_db_version and run V1 + V2.
	}

	// Run pending migrations (skips already-applied ones).
	if err := goose.Up(s.DB.DB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
