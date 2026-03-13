ALTER TABLE contacts
  ADD COLUMN IF NOT EXISTS payment_terms text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS debtor_no text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS creditor_no text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS tax_country char(2) NOT NULL DEFAULT 'DE',
  ADD COLUMN IF NOT EXISTS tax_exempt boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_contacts_debtor_no ON contacts(debtor_no);
CREATE INDEX IF NOT EXISTS idx_contacts_creditor_no ON contacts(creditor_no);
