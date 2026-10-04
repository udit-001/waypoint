-- +goose Up
-- Scrapers permanently opted out of autopilot (WP-175). JSON array of
-- scraper ids; empty = nothing opted out. The application also adds this
-- column on first use for databases that predate this migration.
ALTER TABLE settings ADD COLUMN autopilot_disabled_scrapers TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite doesn't support DROP COLUMN before 3.35.0.
