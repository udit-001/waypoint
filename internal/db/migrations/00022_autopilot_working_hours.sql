-- +goose Up
-- Working-hours cadence (WP-196): the part of the day autopilot works,
-- minutes from midnight, and the in-window firing rate, minutes.
ALTER TABLE settings ADD COLUMN autopilot_work_start INTEGER NOT NULL DEFAULT 540;
ALTER TABLE settings ADD COLUMN autopilot_work_end INTEGER NOT NULL DEFAULT 1080;
ALTER TABLE settings ADD COLUMN autopilot_working_cadence INTEGER NOT NULL DEFAULT 45;

-- +goose Down
-- SQLite doesn't support DROP COLUMN before 3.35.0.
