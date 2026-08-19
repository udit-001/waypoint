-- +goose Up
-- No-op: zen_api_key is created by the application. The column is added
-- on first use via a safe ADD COLUMN IF NOT EXISTS equivalent.
SELECT 1;

-- +goose Down
-- SQLite doesn't support DROP COLUMN.
