-- +goose Up
-- Normalize postings.first_seen to RFC3339. Rows written before this
-- migration carried date-only strings ("2026-08-22"), which sort
-- arbitrarily against full timestamps and made backlog sampling
-- near-random within a day.
UPDATE postings
   SET first_seen = substr(first_seen, 1, 10) || 'T00:00:00Z'
 WHERE length(first_seen) = 10;

-- +goose Down
SELECT 1;
