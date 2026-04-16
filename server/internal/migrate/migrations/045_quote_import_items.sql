CREATE TABLE IF NOT EXISTS quote_import_items (
    id UUID PRIMARY KEY,
    import_id UUID NOT NULL REFERENCES quote_imports(id) ON DELETE CASCADE,
    position_no TEXT NOT NULL,
    outline_no TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL,
    qty NUMERIC(15,3) NOT NULL DEFAULT 0,
    unit TEXT NOT NULL DEFAULT '',
    is_optional BOOLEAN NOT NULL DEFAULT false,
    parser_hint TEXT NOT NULL DEFAULT '',
    review_status TEXT NOT NULL DEFAULT 'pending',
    review_note TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_quote_import_items_import_sort
    ON quote_import_items (import_id, sort_order ASC, id ASC);
