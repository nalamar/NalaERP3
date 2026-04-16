CREATE TABLE IF NOT EXISTS quote_import_item_links (
    id UUID PRIMARY KEY,
    quote_import_item_id UUID NOT NULL REFERENCES quote_import_items(id) ON DELETE CASCADE,
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    quote_item_id UUID NOT NULL REFERENCES quote_items(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (quote_import_item_id),
    UNIQUE (quote_item_id)
);

CREATE INDEX IF NOT EXISTS idx_quote_import_item_links_quote
    ON quote_import_item_links (quote_id, quote_item_id);
