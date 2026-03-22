ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS accepted_at timestamptz,
    ADD COLUMN IF NOT EXISTS linked_invoice_out_id UUID REFERENCES invoices_out(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_quotes_linked_invoice_out ON quotes(linked_invoice_out_id);

ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS source_quote_id UUID REFERENCES quotes(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_out_source_quote ON invoices_out(source_quote_id);
