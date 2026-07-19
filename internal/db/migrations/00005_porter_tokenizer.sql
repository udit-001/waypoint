-- +goose Up
-- V5: Switch FTS5 tokenizer to 'porter unicode61' for stem-aware search.
--
-- The default unicode61 tokenizer only matches exact tokens — 'managers'
-- doesn't match 'manager'. The porter tokenizer applies Porter stemming
-- so both stem to 'manag' and match. Combined with the prefix '*' query
-- from buildFTSQuery, this gives: 'mana' → 'mana*' → manager/managers/
-- management.
--
-- Mirrors learn-tool's migration 00012_porter_tokenizer.sql.
-- Irreversible (Down would require rebuilding without the tokenizer, but
-- there's no reason to go back).

-- ── jobs_fts ──────────────────────────────────────────
-- Drop triggers + table, recreate with porter tokenizer, rebuild index.
-- Can't use FTS5 'rebuild' command because the 'category' column is
-- derived from a JOIN on categories, not a direct jobs column.

DROP TRIGGER IF EXISTS jobs_ai;
DROP TRIGGER IF EXISTS jobs_ad;
DROP TRIGGER IF EXISTS jobs_au;
DROP TABLE IF EXISTS jobs_fts;

CREATE VIRTUAL TABLE jobs_fts USING fts5(
    company, position, notes, location, contact, category,
    content=jobs, content_rowid=id,
    tokenize = 'porter unicode61'
);

INSERT INTO jobs_fts(rowid, company, position, notes, location, contact, category)
SELECT j.id, j.company, j.position, j.notes, j.location, j.contact,
       COALESCE(c.name, '')
FROM jobs j LEFT JOIN categories c ON j.category_id = c.id;

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

-- ── artifacts_fts ─────────────────────────────────────
-- Columns (title, skill_id) map directly to the artifacts table, so we
-- can use FTS5's 'rebuild' command.

DROP TRIGGER IF EXISTS artifacts_ai;
DROP TRIGGER IF EXISTS artifacts_ad;
DROP TRIGGER IF EXISTS artifacts_au;
DROP TABLE IF EXISTS artifacts_fts;

CREATE VIRTUAL TABLE artifacts_fts USING fts5(
    title, skill_id,
    content=artifacts, content_rowid=id,
    tokenize = 'porter unicode61'
);

INSERT INTO artifacts_fts(artifacts_fts) VALUES('rebuild');

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

-- +goose Down
