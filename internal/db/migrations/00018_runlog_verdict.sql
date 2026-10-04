-- +goose Up
-- WP-178: the runlog's verdict + per-source evidence. The verdict is the
-- answer ("why did nothing show up?"); counts are the evidence stored
-- under it, never instead of it.
ALTER TABLE autopilot_runs ADD COLUMN verdict TEXT NOT NULL DEFAULT '';
ALTER TABLE autopilot_runs ADD COLUMN per_source TEXT NOT NULL DEFAULT '[]';
-- One name per concept across UI, JSON, and docs: the record's error list
-- is stageErrors.
ALTER TABLE autopilot_runs RENAME COLUMN errors TO stage_errors;

-- A posting's origin source (scraper id), so per-source outcomes stay
-- attributable after a posting leaves its sweep cycle for the backlog.
ALTER TABLE postings ADD COLUMN source TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE postings DROP COLUMN source;
ALTER TABLE autopilot_runs RENAME COLUMN stage_errors TO errors;
ALTER TABLE autopilot_runs DROP COLUMN per_source;
ALTER TABLE autopilot_runs DROP COLUMN verdict;
