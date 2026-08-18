-- +goose Up
-- V8: Postings ledger — scrape_staging becomes postings (WP-137).
-- The staging table is renamed and re-conceptualized as the canonical
-- postings ledger: one row per URL, the world's data, never deleted.
-- Status vocabulary grows to new | shortlisted | dismissed | promoted;
-- the legacy "imported" status is renamed "promoted".

ALTER TABLE scrape_staging RENAME TO postings;

-- RENAME keeps the old indexes attached under their old names; recreate
-- them under posting names and drop the stale ones.
CREATE INDEX IF NOT EXISTS idx_postings_status     ON postings(status);
CREATE INDEX IF NOT EXISTS idx_postings_first_seen ON postings(first_seen);
DROP INDEX IF EXISTS idx_scrape_staging_status;
DROP INDEX IF EXISTS idx_scrape_staging_first_seen;

-- "imported" is now "promoted".
UPDATE postings SET status = 'promoted' WHERE status = 'imported';

-- +goose Down
UPDATE postings SET status = 'imported' WHERE status = 'promoted';
DROP INDEX IF EXISTS idx_postings_status;
DROP INDEX IF EXISTS idx_postings_first_seen;
ALTER TABLE postings RENAME TO scrape_staging;
CREATE INDEX IF NOT EXISTS idx_scrape_staging_status     ON scrape_staging(status);
CREATE INDEX IF NOT EXISTS idx_scrape_staging_first_seen  ON scrape_staging(first_seen);
