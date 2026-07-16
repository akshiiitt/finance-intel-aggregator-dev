DROP INDEX IF EXISTS idx_processed_investors_gin;
DROP INDEX IF EXISTS idx_processed_companies_gin;

ALTER TABLE processed_items
    ALTER COLUMN companies DROP NOT NULL,
    ALTER COLUMN companies DROP DEFAULT,
    ALTER COLUMN investors DROP NOT NULL,
    ALTER COLUMN investors DROP DEFAULT;

ALTER TABLE processed_items
    ALTER COLUMN companies TYPE TEXT USING jsonb_to_csv(companies),
    ALTER COLUMN investors TYPE TEXT USING jsonb_to_csv(investors);

DROP FUNCTION IF EXISTS jsonb_to_csv(JSONB);
