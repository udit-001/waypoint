-- +goose Up
-- Live cycle progress: the in-flight run row carries the stage facts the
-- web renders ("fetching jobs from Instem — board 3 of 21"). Stage is one
-- of the cycle's fixed stage ids (discover/select/sweep/read/curate);
-- stage_of is a "k/n" counter pair; stage_label the per-item fact (the
-- source display name). Cleared when the run closes — a finished row is
-- the run record only.
ALTER TABLE autopilot_runs ADD COLUMN stage TEXT NOT NULL DEFAULT '';
ALTER TABLE autopilot_runs ADD COLUMN stage_of TEXT NOT NULL DEFAULT '';
ALTER TABLE autopilot_runs ADD COLUMN stage_label TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE autopilot_runs DROP COLUMN stage_label;
ALTER TABLE autopilot_runs DROP COLUMN stage_of;
ALTER TABLE autopilot_runs DROP COLUMN stage;
