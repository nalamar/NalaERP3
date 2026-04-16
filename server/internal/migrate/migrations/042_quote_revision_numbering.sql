ALTER TABLE quotes
    DROP CONSTRAINT IF EXISTS quotes_nummer_key;

CREATE INDEX IF NOT EXISTS idx_quotes_nummer ON quotes(nummer);
