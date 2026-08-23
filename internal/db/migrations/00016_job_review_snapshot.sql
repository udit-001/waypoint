-- +goose Up
-- Snapshot of the autopilot review (overview/note/score/reasons) taken
-- at promote time, so a job's "Why your autopilot picked this" panel
-- survives ledger pruning.
ALTER TABLE jobs ADD COLUMN review_json TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
