-- +goose Up
-- V6: Add the curation-brief columns to profile.
--
-- Three buckets, all owned by the singleton profile row:
--   facts      current_location, seniority
--   constraints visa_sponsorship, salary_floor
--   preferences remote, location_preference, companies, avoid_companies,
--               keywords, dealbreakers
--
-- Array-valued preferences follow the existing skills/experience pattern:
-- a TEXT column storing a JSON array. Single-valued fields are scalars.
-- salary_floor stores {region, amount}; currency is derived at read time
-- (not stored) — see the spec WP-105.
--
-- Irreversible-ish: Down drops the columns. Safe to revert since these are
-- additive preference fields; nothing else depends on them yet.

ALTER TABLE profile ADD COLUMN current_location TEXT NOT NULL DEFAULT '';
ALTER TABLE profile ADD COLUMN seniority TEXT NOT NULL DEFAULT '';
ALTER TABLE profile ADD COLUMN visa_sponsorship TEXT NOT NULL DEFAULT '';
ALTER TABLE profile ADD COLUMN salary_floor TEXT NOT NULL DEFAULT '';
ALTER TABLE profile ADD COLUMN remote TEXT NOT NULL DEFAULT '';
ALTER TABLE profile ADD COLUMN location_preference TEXT NOT NULL DEFAULT '[]';
ALTER TABLE profile ADD COLUMN companies TEXT NOT NULL DEFAULT '[]';
ALTER TABLE profile ADD COLUMN avoid_companies TEXT NOT NULL DEFAULT '[]';
ALTER TABLE profile ADD COLUMN keywords TEXT NOT NULL DEFAULT '[]';
ALTER TABLE profile ADD COLUMN dealbreakers TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE profile DROP COLUMN dealbreakers;
ALTER TABLE profile DROP COLUMN keywords;
ALTER TABLE profile DROP COLUMN avoid_companies;
ALTER TABLE profile DROP COLUMN companies;
ALTER TABLE profile DROP COLUMN location_preference;
ALTER TABLE profile DROP COLUMN remote;
ALTER TABLE profile DROP COLUMN salary_floor;
ALTER TABLE profile DROP COLUMN visa_sponsorship;
ALTER TABLE profile DROP COLUMN seniority;
ALTER TABLE profile DROP COLUMN current_location;
