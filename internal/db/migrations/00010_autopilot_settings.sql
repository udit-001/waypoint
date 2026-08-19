-- +goose Up
-- +goose NO TRANSACTION
-- Add autopilot columns. The application also adds these on first use
-- to handle databases that predate this migration.
SELECT 1;

-- +goose Down
-- SQLite doesn't support DROP COLUMN before 3.35.0.
