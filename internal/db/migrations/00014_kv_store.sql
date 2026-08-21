-- +goose Up
-- V14: Key-value store for small durable state that doesn't warrant a
-- table of its own (WP-149). First consumer: `board_last_swept.<name>`
-- rows recording how each board's last sweep ended — the Companies
-- page's trust strip. Values are JSON blobs owned by their feature.

CREATE TABLE IF NOT EXISTS kv (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS kv;
