-- +goose Up
CREATE TABLE IF NOT EXISTS autopilot_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    duration_ms INTEGER,
    postings_new INTEGER DEFAULT 0,
    postings_shortlisted INTEGER DEFAULT 0,
    postings_dismissed INTEGER DEFAULT 0,
    postings_errored INTEGER DEFAULT 0,
    errors TEXT DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS autopilot_runs;
