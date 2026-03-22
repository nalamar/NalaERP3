INSERT INTO pdf_templates(entity)
SELECT 'sales_order'
WHERE NOT EXISTS (SELECT 1 FROM pdf_templates WHERE entity='sales_order');
