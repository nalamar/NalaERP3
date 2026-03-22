ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS linked_invoice_out_id UUID REFERENCES invoices_out(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sales_orders_linked_invoice_out ON sales_orders(linked_invoice_out_id);

UPDATE sales_orders so
SET linked_invoice_out_id = q.linked_invoice_out_id
FROM quotes q
WHERE q.linked_sales_order_id = so.id
  AND q.linked_invoice_out_id IS NOT NULL
  AND so.linked_invoice_out_id IS NULL;
