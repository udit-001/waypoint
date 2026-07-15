-- +goose Up
-- V4: Scrape staging table — persists scraped results in SQLite
-- before promotion to jobs. Replaces the JSON file staging (expand phase:
-- existing scrape commands still use the JSON file; a later ticket switches them).

CREATE TABLE IF NOT EXISTS scrape_staging (
    url         TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    company     TEXT NOT NULL DEFAULT '',
    location    TEXT NOT NULL DEFAULT '',
    date        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    metadata    TEXT NOT NULL DEFAULT '{}',
    first_seen  TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'new'
);

CREATE INDEX IF NOT EXISTS idx_scrape_staging_status     ON scrape_staging(status);
CREATE INDEX IF NOT EXISTS idx_scrape_staging_first_seen  ON scrape_staging(first_seen);
