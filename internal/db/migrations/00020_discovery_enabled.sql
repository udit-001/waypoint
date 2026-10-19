-- +goose Up
-- Company-discovery toggle (WP-183). Default ON: the Sources tab toggle
-- only turns it off. The application also adds this column on first use
-- for databases that predate this migration.
ALTER TABLE settings ADD COLUMN discovery_enabled INTEGER NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite doesn't support DROP COLUMN before 3.35.0.
