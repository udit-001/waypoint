-- +goose Up
-- WP-181: auto-promote. A run's discovered-boards count rides the runlog
-- as evidence under the verdict ("+3 discovered boards"), never a
-- verdict input.
ALTER TABLE autopilot_runs ADD COLUMN discovered_boards INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE autopilot_runs DROP COLUMN discovered_boards;
