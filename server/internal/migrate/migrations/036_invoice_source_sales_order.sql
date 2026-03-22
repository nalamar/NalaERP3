ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS source_sales_order_id UUID REFERENCES sales_orders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_out_source_sales_order ON invoices_out(source_sales_order_id);

UPDATE invoices_out i
SET source_sales_order_id = so.id
FROM sales_orders so
WHERE so.linked_invoice_out_id = i.id
  AND i.source_sales_order_id IS NULL;
