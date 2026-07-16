-- companies/investors were comma-separated TEXT — unqueryable and
-- unindexable by design (can't ask "every article mentioning Zepto"
-- without a substring scan). This converts both to real JSONB arrays with
-- GIN indexes, so containment queries (companies @> '["Zepto"]') and
-- iteration (jsonb_array_elements_text) are native.
--
-- jsonb_to_csv() below exists so every existing Go query can keep reading
-- these columns as a ", "-joined string — the API/OpenAPI/frontend contract
-- is unchanged — while the underlying storage, and therefore what SQL can
-- do with it, actually improves.

CREATE OR REPLACE FUNCTION jsonb_to_csv(arr JSONB) RETURNS TEXT AS $$
    SELECT NULLIF(string_agg(value, ', '), '')
    FROM jsonb_array_elements_text(COALESCE(arr, '[]'::jsonb)) AS value;
$$ LANGUAGE SQL IMMUTABLE;

-- ALTER COLUMN ... TYPE ... USING does not allow an inline correlated
-- subquery (Postgres: "cannot use subquery in transform expression"), so
-- the comma-split-and-aggregate logic has to live in a function instead —
-- a plain function CALL is fine in a USING clause, only inline SELECTs aren't.
CREATE OR REPLACE FUNCTION csv_to_jsonb(csv TEXT) RETURNS JSONB AS $$
    SELECT CASE WHEN csv IS NULL OR btrim(csv) = '' THEN '[]'::jsonb
         ELSE COALESCE(
             (SELECT jsonb_agg(elem) FROM (
                 SELECT btrim(x) AS elem FROM unnest(string_to_array(csv, ',')) AS x
             ) s WHERE elem <> ''),
             '[]'::jsonb
         )
    END;
$$ LANGUAGE SQL IMMUTABLE;

ALTER TABLE processed_items
    ALTER COLUMN companies TYPE JSONB USING csv_to_jsonb(companies),
    ALTER COLUMN investors TYPE JSONB USING csv_to_jsonb(investors);

ALTER TABLE processed_items
    ALTER COLUMN companies SET DEFAULT '[]'::jsonb,
    ALTER COLUMN companies SET NOT NULL,
    ALTER COLUMN investors SET DEFAULT '[]'::jsonb,
    ALTER COLUMN investors SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_processed_companies_gin ON processed_items USING gin (companies);
CREATE INDEX IF NOT EXISTS idx_processed_investors_gin ON processed_items USING gin (investors);

DROP FUNCTION csv_to_jsonb(TEXT);
