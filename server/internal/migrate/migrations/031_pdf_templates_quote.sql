-- Seed quote PDF template configuration

INSERT INTO pdf_templates(entity)
SELECT 'quote'
WHERE NOT EXISTS (SELECT 1 FROM pdf_templates WHERE entity='quote');
