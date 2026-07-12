-- +goose Up
-- V3: Add indexes on filtered/joined columns.
-- Every filtered query was doing a full table scan. These indexes cover
-- the columns used in WHERE, JOIN, and dedup queries.

CREATE INDEX IF NOT EXISTS idx_jobs_status      ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_category_id  ON jobs(category_id);
CREATE INDEX IF NOT EXISTS idx_jobs_url          ON jobs(url);
CREATE INDEX IF NOT EXISTS idx_history_job_id    ON history(job_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_job_id  ON artifacts(job_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_skill_id ON artifacts(skill_id);
