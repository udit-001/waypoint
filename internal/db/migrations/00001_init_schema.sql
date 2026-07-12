-- +goose Up
-- V1: Frozen baseline of the original schema (includes seed data).
-- This migration captures the exact state of the database as it existed
-- before goose was introduced. Existing databases are baselined to V1
-- without running this migration; fresh databases run it in full.

CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    company TEXT NOT NULL DEFAULT '',
    position TEXT NOT NULL DEFAULT '',
    date TEXT NOT NULL DEFAULT '',
    applied_date TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'Not Applied',
    category_id INTEGER NOT NULL DEFAULT 1 REFERENCES categories(id),
    salary TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    contact TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    reminder_date TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    from_value TEXT NOT NULL DEFAULT '',
    to_value TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS profile (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    skills TEXT NOT NULL DEFAULT '[]',
    experience TEXT NOT NULL DEFAULT '[]',
    education TEXT NOT NULL DEFAULT '[]',
    industry TEXT NOT NULL DEFAULT '',
    greeting_style TEXT NOT NULL DEFAULT 'formal',
    sign_off TEXT NOT NULL DEFAULT 'Best regards'
);

INSERT OR IGNORE INTO profile (id) VALUES (1);

CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    theme TEXT NOT NULL DEFAULT 'light',
    reminders_enabled INTEGER NOT NULL DEFAULT 1,
    default_view TEXT NOT NULL DEFAULT 'dashboard',
    items_per_page INTEGER NOT NULL DEFAULT 25
);

INSERT OR IGNORE INTO settings (id) VALUES (1);

CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
    company, position, notes, location, contact, category,
    content=jobs, content_rowid=id
);

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

CREATE TABLE IF NOT EXISTS artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id TEXT NOT NULL,
    job_id INTEGER REFERENCES jobs(id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    options TEXT NOT NULL DEFAULT '{}',
    variants TEXT NOT NULL DEFAULT '[]',
    archived INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
    title, skill_id,
    content=artifacts, content_rowid=id
);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS artifacts_ai AFTER INSERT ON artifacts BEGIN
    INSERT INTO artifacts_fts(rowid, title, skill_id) VALUES (new.id, new.title, new.skill_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS artifacts_ad AFTER DELETE ON artifacts BEGIN
    INSERT INTO artifacts_fts(artifacts_fts, rowid, title, skill_id)
    VALUES ('delete', old.id, old.title, old.skill_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS artifacts_au AFTER UPDATE ON artifacts BEGIN
    INSERT INTO artifacts_fts(artifacts_fts, rowid, title, skill_id)
    VALUES ('delete', old.id, old.title, old.skill_id);
    INSERT INTO artifacts_fts(rowid, title, skill_id) VALUES (new.id, new.title, new.skill_id);
END;
-- +goose StatementEnd

-- Seed categories (frozen as-is — V2 does not remove them from existing DBs)
INSERT OR IGNORE INTO categories (name) VALUES ('General');
INSERT OR IGNORE INTO categories (name) VALUES ('Tech'), ('Finance'), ('Healthcare');
