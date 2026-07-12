-- +goose Up
-- +goose NO TRANSACTION
-- V2: Allow NULL category_id on jobs (eliminate seed dependency).
-- SQLite doesn't support ALTER COLUMN, so we rebuild the jobs table.
--
-- NO TRANSACTION is required because PRAGMA foreign_keys cannot be changed
-- inside a transaction. We disable FKs to prevent DROP TABLE jobs from
-- cascading deletes into the history table (ON DELETE CASCADE).
-- The create-new/copy/drop-old/rename pattern preserves FK references in
-- history and artifacts — they always say REFERENCES jobs(id).

-- Disable FK enforcement during the rebuild
PRAGMA foreign_keys=OFF;

-- Step 1: Create new jobs table with nullable category_id
CREATE TABLE jobs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    company TEXT NOT NULL DEFAULT '',
    position TEXT NOT NULL DEFAULT '',
    date TEXT NOT NULL DEFAULT '',
    applied_date TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'Not Applied',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    salary TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    contact TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    reminder_date TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Step 2: Copy all existing data (no triggers on jobs_new)
INSERT INTO jobs_new SELECT * FROM jobs;

-- Step 3: Drop the old jobs table (no cascade deletes because FKs are off)
DROP TABLE jobs;

-- Step 4: Rename new table to jobs (FK refs in history/artifacts now valid)
ALTER TABLE jobs_new RENAME TO jobs;

-- Step 5: Drop and recreate the FTS table so its content reference
-- points to the new jobs table
DROP TABLE IF EXISTS jobs_fts;

CREATE VIRTUAL TABLE jobs_fts USING fts5(
    company, position, notes, location, contact, category,
    content=jobs, content_rowid=id
);

-- Step 6: Recreate FTS sync triggers on the new jobs table
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS jobs_ai AFTER INSERT ON jobs BEGIN
    INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
    VALUES (new.id, new.company, new.position, new.notes, new.location, new.contact,
            (SELECT name FROM categories WHERE id = new.category_id));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS jobs_ad AFTER DELETE ON jobs BEGIN
    INSERT INTO jobs_fts(jobs_fts, rowid, company, position, notes, location, contact, category)
    VALUES ('delete', old.id, old.company, old.position, old.notes, old.location, old.contact,
            (SELECT name FROM categories WHERE id = old.category_id));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS jobs_au AFTER UPDATE ON jobs BEGIN
    INSERT INTO jobs_fts(jobs_fts, rowid, company, position, notes, location, contact, category)
    VALUES ('delete', old.id, old.company, old.position, old.notes, old.location, old.contact,
            (SELECT name FROM categories WHERE id = old.category_id));
    INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
    VALUES (new.id, new.company, new.position, new.notes, new.location, new.contact,
            (SELECT name FROM categories WHERE id = new.category_id));
END;
-- +goose StatementEnd

-- Step 7: Populate FTS index from the new jobs table.
-- Can't use 'rebuild' because the FTS 'category' column is derived from
-- a JOIN on categories, not a direct column in the jobs table.
INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
SELECT j.id, j.company, j.position, j.notes, j.location, j.contact, c.name
FROM jobs j LEFT JOIN categories c ON j.category_id = c.id;

-- Re-enable FK enforcement
PRAGMA foreign_keys=ON;
