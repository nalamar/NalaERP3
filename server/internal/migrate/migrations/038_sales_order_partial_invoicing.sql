ALTER TABLE invoice_out_items
    ADD COLUMN IF NOT EXISTS source_sales_order_item_id UUID REFERENCES sales_order_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_out_items_source_sales_order_item
    ON invoice_out_items(source_sales_order_item_id);

UPDATE invoice_out_items ioi
SET source_sales_order_item_id = soi.id
FROM invoices_out io
JOIN sales_order_items soi
  ON soi.sales_order_id = io.source_sales_order_id
 AND soi.position = ioi.position
WHERE ioi.invoice_id = io.id
  AND io.source_sales_order_id IS NOT NULL
  AND ioi.source_sales_order_item_id IS NULL;
