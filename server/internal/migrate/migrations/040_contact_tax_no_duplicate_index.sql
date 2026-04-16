CREATE INDEX IF NOT EXISTS idx_contacts_tax_no_norm
ON contacts ((lower(btrim(tax_no))))
WHERE btrim(tax_no) <> '';
