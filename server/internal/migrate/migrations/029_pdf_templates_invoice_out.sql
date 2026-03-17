-- Seed additional PDF template entities for accounting documents

INSERT INTO pdf_templates(entity)
SELECT 'invoice_out'
WHERE NOT EXISTS (SELECT 1 FROM pdf_templates WHERE entity='invoice_out');
