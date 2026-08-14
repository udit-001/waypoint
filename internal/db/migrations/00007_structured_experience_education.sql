-- +goose Up
-- V7: Drop the badly-planned email style columns and move experience/education
-- to structured objects.
--
--   greeting_style, sign_off — never used for the search brief; removed.
--   experience / education   — stay TEXT columns holding a JSON array, but the
--                              array now holds objects:
--                                experience: [{title, company, start, end}]
--                                education:  [{institution, degree, start, end}]
--                              dates are partial ISO (YYYY-MM or YYYY); an empty
--                              end means "present". Legacy flat-string arrays
--                              are upgraded on read (see ParseExperience), not
--                              in SQL — free text can't be parsed reliably here.
--
-- Down re-adds the columns with their original defaults.

ALTER TABLE profile DROP COLUMN greeting_style;
ALTER TABLE profile DROP COLUMN sign_off;

-- +goose Down
ALTER TABLE profile ADD COLUMN greeting_style TEXT NOT NULL DEFAULT 'formal';
ALTER TABLE profile ADD COLUMN sign_off TEXT NOT NULL DEFAULT 'Best regards';
