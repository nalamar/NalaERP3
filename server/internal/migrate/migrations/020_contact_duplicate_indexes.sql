CREATE INDEX IF NOT EXISTS idx_contacts_name_norm
ON contacts ((lower(btrim(name))));

CREATE INDEX IF NOT EXISTS idx_contacts_email_norm
ON contacts ((lower(btrim(email))))
WHERE btrim(email) <> '';

CREATE INDEX IF NOT EXISTS idx_contacts_vat_norm
ON contacts ((lower(btrim(vat_id))))
WHERE btrim(vat_id) <> '';

CREATE INDEX IF NOT EXISTS idx_contacts_debtor_norm
ON contacts ((lower(btrim(debtor_no))))
WHERE btrim(debtor_no) <> '';

CREATE INDEX IF NOT EXISTS idx_contacts_creditor_norm
ON contacts ((lower(btrim(creditor_no))))
WHERE btrim(creditor_no) <> '';
