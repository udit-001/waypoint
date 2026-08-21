-- +goose Up
-- V13: Company discovery candidates (WP-150).
-- One row per discovered company: the discovery pipeline's durable
-- output. Re-runs upsert by unique name; status vocabulary is
-- suggested | added | dismissed (add/dismiss lands in WP-151).
-- Boards is a JSON array of {provider,url} objects.

CREATE TABLE company_candidates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    domain TEXT NOT NULL DEFAULT '',
    boards TEXT NOT NULL DEFAULT '[]',
    facet TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'suggested'
        CHECK (status IN ('suggested', 'added', 'dismissed')),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_company_candidates_name ON company_candidates(name);

-- +goose Down
DROP INDEX IF EXISTS idx_company_candidates_name;
DROP TABLE IF EXISTS company_candidates;
