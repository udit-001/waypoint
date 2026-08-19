-- +goose Up
CREATE TABLE IF NOT EXISTS change_events (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    kind  TEXT NOT NULL,
    at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_change_events_id ON change_events(id);

-- +goose Down
DROP TABLE IF EXISTS change_events;
