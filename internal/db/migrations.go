package db

import (
	"embed"
	"fmt"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// backupDB copies the database file to <path>.bak. Best-effort — errors
// are silently ignored since the backup is a safety net, not a requirement.
// A WAL checkpoint is run first so the copy captures all committed data.
//
// TEMPORARY: This protects against the V2 NO TRANSACTION migration's
// failure window. Once all users have V2 applied, this code never fires
// (gated on version < 2) and can be removed.
func backupDB(db *sqlx.DB, dbPath string) {
	// Checkpoint WAL into main DB file so the copy is consistent
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	src, err := os.Open(dbPath)
	if err != nil {
		return
	}
	defer src.Close()

	dst, err := os.Create(dbPath + ".bak")
	if err != nil {
		return
	}
	defer dst.Close()

	_, _ = io.Copy(dst, src)
}

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

	// Backup before migration if V2 hasn't been applied yet.
	// TEMPORARY — remove this block once all users are on V2.
	needsBackup := true
	if gooseTableExists {
		var version int
		if err := s.Get(&version, "SELECT max(version_id) FROM goose_db_version WHERE is_applied = 1"); err == nil && version >= 2 {
			needsBackup = false
		}
	}
	if needsBackup {
		backupDB(s.DB, dbPath)
	}

	// Run pending migrations (skips already-applied ones).
	if err := goose.Up(s.DB.DB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
