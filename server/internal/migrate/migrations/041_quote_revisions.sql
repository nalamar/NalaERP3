ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS root_quote_id UUID REFERENCES quotes(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS revision_no integer NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS superseded_by_quote_id UUID REFERENCES quotes(id) ON DELETE SET NULL;

UPDATE quotes
SET root_quote_id = id
WHERE root_quote_id IS NULL;

ALTER TABLE quotes
    ALTER COLUMN root_quote_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_quotes_root_quote_id ON quotes(root_quote_id);
CREATE INDEX IF NOT EXISTS idx_quotes_superseded_by_quote_id ON quotes(superseded_by_quote_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_quotes_root_revision_no ON quotes(root_quote_id, revision_no);
