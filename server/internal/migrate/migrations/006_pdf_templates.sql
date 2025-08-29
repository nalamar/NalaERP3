-- PDF Templates settings

CREATE TABLE IF NOT EXISTS pdf_templates (
  entity text PRIMARY KEY,
  header_text text NOT NULL DEFAULT '',
  footer_text text NOT NULL DEFAULT '',
  logo_doc_id text,
  bg_first_doc_id text,
  bg_other_doc_id text,
  top_first_mm numeric(10,2) NOT NULL DEFAULT 30,
  top_other_mm numeric(10,2) NOT NULL DEFAULT 20,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Seed for purchase_order if not exists
INSERT INTO pdf_templates(entity)
SELECT 'purchase_order'
WHERE NOT EXISTS (SELECT 1 FROM pdf_templates WHERE entity='purchase_order');

